package agreementbot

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/agreementbot/persistence"
	"github.com/open-horizon/anax/agreementbot/secrets"
	"github.com/open-horizon/anax/compcheck"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"sync"
	"time"
)

// The main component which holds secret updates for the governance functions.
type SecretUpdateManager struct {
	PendingUpdates []*events.SecretUpdates // Secret update events that need to be processed
	PULock         sync.Mutex              // The lock that protects the list of pending secret updates.
}

func NewSecretUpdateManager() *SecretUpdateManager {
	return new(SecretUpdateManager)
}

func (sm *SecretUpdateManager) GetNextUpdateEvent() (su *events.SecretUpdates) {

	sm.PULock.Lock()
	defer sm.PULock.Unlock()

	if len(sm.PendingUpdates) == 0 {
		return nil
	}

	su, sm.PendingUpdates = sm.PendingUpdates[0], sm.PendingUpdates[1:]
	return

}

func (sm *SecretUpdateManager) SetUpdateEvent(ev *events.SecretUpdates) {

	sm.PULock.Lock()
	defer sm.PULock.Unlock()

	sm.PendingUpdates = append(sm.PendingUpdates, ev)
}

// Examine all the managed secrets in the DB, to see if any of them have been updated in the secret provider.
func (sm *SecretUpdateManager) CheckForUpdates(secretProvider secrets.AgbotSecrets, db persistence.AgbotDatabase) (*events.SecretUpdates, error) {

	if secretProvider == nil || !secretProvider.IsReady() {
		return nil, nil
	}

	// Get the list of managed secrets from the DB, for all policies and patterns.
	polSecretNames, err := db.GetManagedPolicySecretNames("", "")
	if err != nil {
		return nil, errors.New(smlogString(fmt.Sprintf("Error retrieving managed policy secret list, error: %v", err)))
	}

	patSecretNames, err := db.GetManagedPatternSecretNames("", "")
	if err != nil {
		return nil, errors.New(smlogString(fmt.Sprintf("Error retrieving managed pattern secret list, error: %v", err)))
	}

	// Combine both lists, remove duplicates.
	secretNames := cutil.MergeSlices(polSecretNames, patSecretNames)

	// If there are no secrets to manage, just return.
	if secretNames == nil || len(secretNames) == 0 {
		return nil, nil
	}

	// Collect up any secret updates in this object, which will be sent as an event to the internal bus.
	secretUpdates := events.NewSecretUpdates()

	// For each secret, check to see if it has changed since it was last checked.
	for _, fullSecretName := range secretNames {

		secretOrg := exchange.GetOrg(fullSecretName)
		secretUser, secretName, err := compcheck.ParseVaultSecretName(exchange.GetId(fullSecretName), nil)
		if err != nil {
			glog.Errorf(smlogString(fmt.Sprintf("Error parsing secret %s, error: %v", fullSecretName, err)))
			continue
		}

		glog.V(5).Infof(smlogString(fmt.Sprintf("Checking for changes to secret %s", fullSecretName)))

		// All secrets that are referenced by a policy or pattern are in the secret update tables, but some of these secrets
		// might not exist yet.
		secretMetadata, err := secretProvider.GetSecretMetadata(secretOrg, secretUser, secretName)
		if err != nil {
			// For secrets that dont exist yet, just ignore them.
			glog.Warningf(smlogString(fmt.Sprintf("Error retrieving metadata for secret %s for user %s in org %s metadata, error: %v", secretName, secretUser, secretOrg, err)))
			continue
		}

		glog.V(5).Infof(smlogString(fmt.Sprintf("Secret %s metadata: %v", fullSecretName, secretMetadata)))

		// Get a list of policies that have a secret which has been updated.
		policyNames, err := db.GetPoliciesWithUpdatedSecrets(secretOrg, exchange.GetId(fullSecretName), secretMetadata.UpdateTime)
		if err != nil {
			glog.Errorf(smlogString(fmt.Sprintf("Error checking policies for updated secret %s", fullSecretName)))
			continue
		}

		// If there are policies returned, then it means that the policy references the secret and the secret has been updated.
		if len(policyNames) != 0 {
			su := events.NewSecretUpdate(secretOrg, exchange.GetId(fullSecretName), secretMetadata.UpdateTime, policyNames, []string{})
			secretUpdates.AddSecretUpdate(su)
			glog.V(5).Infof(smlogString(fmt.Sprintf("Policies affected by %s, %v", fullSecretName, policyNames)))
		}

		// Get a list of patterns that have a secret which has been updated.
		patternNames, err := db.GetPatternsWithUpdatedSecrets(secretOrg, exchange.GetId(fullSecretName), secretMetadata.UpdateTime)
		if err != nil {
			glog.Errorf(smlogString(fmt.Sprintf("Error checking patterns for updated secret %s", fullSecretName)))
			continue
		}

		// If there are patterns returned, then it means that the secret has been updated.
		if len(patternNames) != 0 {
			su := events.NewSecretUpdate(secretOrg, exchange.GetId(fullSecretName), secretMetadata.UpdateTime, []string{}, patternNames)
			secretUpdates.AddSecretUpdate(su)
			glog.V(5).Infof(smlogString(fmt.Sprintf("Patterns affected by %s, %v", fullSecretName, patternNames)))
		}

	}

	return secretUpdates, nil
}

// When policies are added, changed or deleted, the list of managed secrets in the DB might need to be updated.
func (sm *SecretUpdateManager) UpdatePolicies(org string, exchPolsMetadata map[string]exchange.ExchangeBusinessPolicy, secretProvider secrets.AgbotSecrets, db persistence.AgbotDatabase) error {

	if secretProvider == nil || !secretProvider.IsReady() {
		return nil
	}

	// Get a list of all the policies that have secrets and then remove deleted policies from
	// the secrets DB.  This function returns org qualified policy names.
	policies, err := db.GetPoliciesInOrg(org)
	if err != nil {
		return errors.New(smlogString(fmt.Sprintf("unable to retrieve policies for org %s, error: %v", org, err)))
	}

	// Look for policies in the DB that are no longer published to the exchange. If any are found, remove all the secret rows
	// for that policy.
	for _, polName := range policies {
		if _, ok := exchPolsMetadata[polName]; !ok {
			glog.V(5).Infof(smlogString(fmt.Sprintf("deleting all secrets for %s", polName)))
			err = db.DeleteSecretsForPolicy(org, exchange.GetId(polName))
			if err != nil {
				glog.Errorf(smlogString(fmt.Sprintf("unable to delete %s from secrets DB, error: %v", polName, err)))
			}
		}
	}

	// Load the secrets table with all secrets used by deployment policies. Remove secrets that are no longer in use. A policy
	// could have changed, removing or adding a reference to a secret.
	for policyName, dpol := range exchPolsMetadata {

		// Get the list of managed secrets for this policy from the DB.
		secretNames, err := db.GetManagedPolicySecretNames(org, exchange.GetId(policyName))
		if err != nil {
			glog.Errorf(smlogString(fmt.Sprintf("Error retrieving managed secret list for %s, error: %v", policyName, err)))
			continue
		}

		// Keep track of which secrets are being referenced so that unused secrets can be removed at the end.
		referencedSecrets := make(map[string]bool)

		// Iterate the list of secret bindings in the policy to get the secret manager secret names.
		for _, sb := range dpol.SecretBinding {
			// Iterate each bound secret
			for _, bs := range sb.Secrets {
				// Extract the secret manager secret name
				_, secretFullName := bs.GetBinding()
				referencedSecrets[fmt.Sprintf("%s/%s", org, secretFullName)] = true

				secretUser, secretName, err := compcheck.ParseVaultSecretName(secretFullName, nil)
				if err != nil {
					glog.Errorf(smlogString(fmt.Sprintf("unable to parse secret name %s, error: %v", secretFullName, err)))
					continue
				}

				pName := exchange.GetId(policyName)

				// Use now as the last update time for secrets that dont exist yet.
				secretLastUpdateTime := time.Now().Unix()

				// Get the secret's metadata, if it exists
				sm, err := secretProvider.GetSecretMetadata(org, secretUser, secretName)
				if err != nil {
					// The secret should be stored in the table even if it doesnt exist, so that if it is created later
					// changes to it will be recognized.
					glog.Warningf(smlogString(fmt.Sprintf("unable to retrieve metadata for %s %s, error: %v", org, secretFullName, err)))
					//continue
				} else {
					secretLastUpdateTime = sm.UpdateTime
				}

				glog.V(5).Infof(smlogString(fmt.Sprintf("storing managed secret %v %v from %v/%v", org, secretFullName, org, pName)))

				// Only secrets that have never been referenced before are added to the DB. DB rows that already exist will not be updated.
				err = db.AddManagedPolicySecret(org, secretFullName, org, pName, secretLastUpdateTime)
				if err != nil {
					glog.Errorf(smlogString(fmt.Sprintf("unable to persist secret %v %v from %v/%v, error: %v", org, secretFullName, org, pName, err)))
				}
			}
		}

		// Look for unreferenced secrets and remove them.
		for _, secretName := range secretNames {
			if _, ok := referencedSecrets[secretName]; !ok {

				glog.V(5).Infof(smlogString(fmt.Sprintf("deleting managed secret %s from %s because it is no longer used", secretName, policyName)))
				err = db.DeletePolicySecret(exchange.GetOrg(secretName), exchange.GetId(secretName), org, exchange.GetId(policyName))
				if err != nil {
					glog.Errorf(smlogString(fmt.Sprintf("unable to delete %s from secrets DB, error: %v", secretName, err)))
				}
			}
		}
	}

	return nil

}

func (sm *SecretUpdateManager) UpdatePatterns(org string, exchPatternMetadata map[string]exchange.Pattern, secretProvider secrets.AgbotSecrets, db persistence.AgbotDatabase) error {

	if secretProvider == nil || !secretProvider.IsReady() {
		return nil
	}

	// Get a list of all the patterns that have secrets and then remove deleted patterns from
	// the secrets DB.  This function returns org qualified pattern names.
	patterns, err := db.GetPatternsInOrg(org)
	if err != nil {
		return errors.New(smlogString(fmt.Sprintf("unable to retrieve patterns for org %s, error: %v", org, err)))
	}

	// Look for patterns in the DB that are no longer published to the exchange. If any are found, remove all the secret rows
	// for that pattern.
	for _, patName := range patterns {
		if _, ok := exchPatternMetadata[patName]; !ok {
			glog.V(5).Infof(smlogString(fmt.Sprintf("deleting all secrets for %s", patName)))
			err = db.DeleteSecretsForPattern(org, exchange.GetId(patName))
			if err != nil {
				glog.Errorf(smlogString(fmt.Sprintf("unable to delete %s from secrets DB, error: %v", patName, err)))
			}
		}
	}

	// Load the secrets table with all secrets used by patterns. Remove secrets that are no longer in use. A pattern
	// could have changed, removing or adding a reference to a secret.
	for patName, pat := range exchPatternMetadata {

		// Get the list of managed secrets for this policy from the DB.
		secretNames, err := db.GetManagedPatternSecretNames(org, exchange.GetId(patName))
		if err != nil {
			glog.Errorf(smlogString(fmt.Sprintf("Error retrieving managed secret list for %s, error: %v", patName, err)))
			continue
		}

		// Keep track of which secrets are being referenced so that unused secrets can be removed at the end.
		referencedSecrets := make(map[string]bool)

		// Iterate the list of secret bindings in the policy to get the secret manager secret names.
		for _, sb := range pat.SecretBinding {
			// Iterate each bound secret
			for _, bs := range sb.Secrets {
				// Extract the secret manager secret name
				_, secretFullName := bs.GetBinding()
				referencedSecrets[fmt.Sprintf("%s/%s", org, secretFullName)] = true

				secretUser, secretName, err := compcheck.ParseVaultSecretName(secretFullName, nil)
				if err != nil {
					glog.Errorf(smlogString(fmt.Sprintf("unable to parse secret name %s, error: %v", secretFullName, err)))
					continue
				}

				pName := exchange.GetId(patName)

				// Use now as the last update time for secrets that dont exist yet.
				secretLastUpdateTime := time.Now().Unix()

				// Get the secret's metadata, if it exists
				sm, err := secretProvider.GetSecretMetadata(org, secretUser, secretName)
				if err != nil {
					// The secret should be stored in the table even if it doesnt exist, so that if it is created later
					// changes to it will be recognized.
					glog.Warningf(smlogString(fmt.Sprintf("unable to retrieve metadata for %s %s, error: %v", org, secretFullName, err)))
					//continue
				} else {
					secretLastUpdateTime = sm.UpdateTime
				}

				glog.V(5).Infof(smlogString(fmt.Sprintf("storing managed secret %v %v from %v/%v", org, secretFullName, org, pName)))

				// Only secrets that have never been referenced before are added to the DB. DB rows that already exist will not be updated.
				err = db.AddManagedPatternSecret(org, secretFullName, org, pName, secretLastUpdateTime)
				if err != nil {
					glog.Errorf(smlogString(fmt.Sprintf("unable to persist secret %v %v from %v/%v, error: %v", org, secretFullName, org, pName, err)))
				}
			}
		}

		// Look for unreferenced secrets and remove them.
		for _, secretName := range secretNames {
			if _, ok := referencedSecrets[secretName]; !ok {

				glog.V(5).Infof(smlogString(fmt.Sprintf("deleting managed secret %s from %s because it is no longer used", secretName, patName)))
				err = db.DeletePatternSecret(exchange.GetOrg(secretName), exchange.GetId(secretName), org, exchange.GetId(patName))
				if err != nil {
					glog.Errorf(smlogString(fmt.Sprintf("unable to delete %s from secrets DB, error: %v", secretName, err)))
				}
			}
		}
	}

	return nil
}

// ==========================================================================================================
// Utility functions

var smlogString = func(v interface{}) string {
	return fmt.Sprintf("SecretsManager %v", v)
}

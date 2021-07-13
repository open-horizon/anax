package bolt

func (db *AgbotBoltDB) AddManagedPolicySecret(secretOrg, secretName, policyOrg, policyName string, updateTime int64) error {
	return nil
}

func (db *AgbotBoltDB) GetManagedPolicySecretNames(policyOrg, policyName string) ([]string, error) {
	return []string{}, nil
}

func (db *AgbotBoltDB) GetPoliciesWithUpdatedSecrets(secretOrg, secretName string, lastUpdate int64) ([]string, error) {
	return []string{}, nil
}

func (db *AgbotBoltDB) SetSecretUpdate(secretOrg, secretName string, secretUpdateTime int64) error {
	return nil
}

func (db *AgbotBoltDB) GetPoliciesInOrg(org string) ([]string, error) {
	return []string{}, nil
}

func (db *AgbotBoltDB) DeleteSecretsForPolicy(polOrg, polName string) error {
	return nil
}

func (db *AgbotBoltDB) DeletePolicySecret(secretOrg, secretName, policyOrg, policyName string) error {
	return nil
}

func (db *AgbotBoltDB) AddManagedPatternSecret(secretOrg, secretName, policyOrg, policyName string, updateTime int64) error {
	return nil
}

func (db *AgbotBoltDB) GetManagedPatternSecretNames(policyOrg, policyName string) ([]string, error) {
	return []string{}, nil
}

func (db *AgbotBoltDB) GetPatternsWithUpdatedSecrets(secretOrg, secretName string, lastUpdate int64) ([]string, error) {
	return []string{}, nil
}

func (db *AgbotBoltDB) GetPatternsInOrg(org string) ([]string, error) {
	return []string{}, nil
}

func (db *AgbotBoltDB) DeleteSecretsForPattern(polOrg, polName string) error {
	return nil
}

func (db *AgbotBoltDB) DeletePatternSecret(secretOrg, secretName, policyOrg, policyName string) error {
	return nil
}
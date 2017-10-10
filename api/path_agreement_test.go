package api

import (
	"flag"
	"github.com/open-horizon/anax/persistence"
	"testing"
)

func init() {
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "7")
	// no need to parse flags, that's done by test framework
}

func Test_FindAgreementsForOutput0(t *testing.T) {

	dir, db, err := setup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// No agreements in the DB yet.

	// Now test the GET /agreements function.
	if agsout, err := FindAgreementsForOutput(db); err != nil {
		t.Errorf("error finding agreements: %v", err)
	} else if len(agsout["agreements"]["active"]) != 0 {
		t.Errorf("expecting 0 active agreements have %v", agsout["agreements"]["active"])
	} else if len(agsout["agreements"]["archived"]) != 0 {
		t.Errorf("expecting 0 archived agreements have %v", agsout["agreements"]["archived"])
	}

}

func Test_FindAgreementsForOutput1(t *testing.T) {

	dir, db, err := setup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// Add 3 agreements to the DB, one for each path in the /agreement GET.
	// Agreement 1 is active, agreement2 is archived, agreement3 is active but terminating.
	if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId1", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement1: %v", err)
	} else if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId2", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement2: %v", err)
	} else if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId3", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement3: %v", err)
	} else if _, err := persistence.ArchiveEstablishedAgreement(db, "agreementId2", "Basic"); err != nil {
		t.Errorf("error archiving agreement2: %v", err)
	} else if _, err := persistence.AgreementStateTerminated(db, "agreementId3", 100, "unit test termination", "Basic"); err != nil {
		t.Errorf("error terminating agreement3: %v", err)
	}

	// Agreements are bootstrapped into the database, now test the GET /agreements function.
	if agsout, err := FindAgreementsForOutput(db); err != nil {
		t.Errorf("error finding agreements: %v", err)
	} else if len(agsout["agreements"]["active"]) != 1 {
		t.Errorf("expecting 1 active agreement have %v", agsout["agreements"]["active"])
	} else if len(agsout["agreements"]["archived"]) != 2 {
		t.Errorf("expecting 2 archived agreements have %v", agsout["agreements"]["archived"])
	}

}

func Test_DeleteAgreement0(t *testing.T) {

	dir, db, err := setup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// Add 3 agreements to the DB, one for each special agreement state.
	// Agreement 1 is active, agreement2 is archived, agreement3 is active but terminating.
	if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId1", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement1: %v", err)
	} else if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId2", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement2: %v", err)
	} else if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId3", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement3: %v", err)
	} else if _, err := persistence.ArchiveEstablishedAgreement(db, "agreementId2", "Basic"); err != nil {
		t.Errorf("error archiving agreement2: %v", err)
	} else if _, err := persistence.AgreementStateTerminated(db, "agreementId3", 100, "unit test termination", "Basic"); err != nil {
		t.Errorf("error terminating agreement3: %v", err)
	}

	// Agreements are bootstrapped into the database, now test the DELETE /agreements function on a non-existant agreement.
	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	if errHandled, msg := DeleteAgreement(errorhandler, "agreementId4", db); !errHandled {
		t.Errorf("expected to receive error")
	} else if _, ok := myError.(*NotFoundError); !ok {
		t.Errorf("expected error of type NotFound, but is %T, %v", myError, myError)
	} else if msg != nil {
		t.Errorf("should not have returned a message object, %v", msg)
	}

}

func Test_DeleteAgreement1(t *testing.T) {

	dir, db, err := setup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// Add 3 agreements to the DB, one for each special agreement state.
	// Agreement 1 is active, agreement2 is archived, agreement3 is active but terminating.
	if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId1", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement1: %v", err)
	} else if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId2", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement2: %v", err)
	} else if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId3", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement3: %v", err)
	} else if _, err := persistence.ArchiveEstablishedAgreement(db, "agreementId2", "Basic"); err != nil {
		t.Errorf("error archiving agreement2: %v", err)
	} else if _, err := persistence.AgreementStateTerminated(db, "agreementId3", 100, "unit test termination", "Basic"); err != nil {
		t.Errorf("error terminating agreement3: %v", err)
	}

	// Agreements are bootstrapped into the database, now test the DELETE /agreements function on an existing agreement.
	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	if errHandled, msg := DeleteAgreement(errorhandler, "agreementId3", db); errHandled {
		t.Errorf("unexpected error (%T) %v", myError, myError)
	} else if msg != nil {
		t.Errorf("should not have returned a message object for terminating agreement")
	}

}

func Test_DeleteAgreement2(t *testing.T) {

	dir, db, err := setup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// Add 3 agreements to the DB, one for each special agreement state.
	// Agreement 1 is active, agreement2 is archived, agreement3 is active but terminating.
	if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId1", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement1: %v", err)
	} else if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId2", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement2: %v", err)
	} else if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId3", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement3: %v", err)
	} else if _, err := persistence.ArchiveEstablishedAgreement(db, "agreementId2", "Basic"); err != nil {
		t.Errorf("error archiving agreement2: %v", err)
	} else if _, err := persistence.AgreementStateTerminated(db, "agreementId3", 100, "unit test termination", "Basic"); err != nil {
		t.Errorf("error terminating agreement3: %v", err)
	}

	// Agreements are bootstrapped into the database, now test the DELETE /agreements function on an existing agreement.
	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	if errHandled, msg := DeleteAgreement(errorhandler, "agreementId1", db); errHandled {
		t.Errorf("unexpected error (%T) %v", myError, myError)
	} else if msg == nil {
		t.Errorf("should have returned a message object")
	}

}

func Test_DeleteAgreement3(t *testing.T) {

	dir, db, err := setup()
	if err != nil {
		t.Error(err)
	}
	defer cleanTestDir(dir)

	// Add 3 agreements to the DB, one for each special agreement state.
	// Agreement 1 is active, agreement2 is archived, agreement3 is active but terminating.
	if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId1", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement1: %v", err)
	} else if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId2", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement2: %v", err)
	} else if _, err := persistence.NewEstablishedAgreement(db, "name1", "agreementId3", "consumerId", "{}", "Basic", 1, []string{"http://sensor.org"}, "signature", "address", "bcType", "bcName", "bcOrg"); err != nil {
		t.Errorf("error writing agreement3: %v", err)
	} else if _, err := persistence.ArchiveEstablishedAgreement(db, "agreementId2", "Basic"); err != nil {
		t.Errorf("error archiving agreement2: %v", err)
	} else if _, err := persistence.AgreementStateTerminated(db, "agreementId3", 100, "unit test termination", "Basic"); err != nil {
		t.Errorf("error terminating agreement3: %v", err)
	}

	// Agreements are bootstrapped into the database, now test the DELETE /agreements function on an existing agreement.
	var myError error
	errorhandler := GetPassThroughErrorHandler(&myError)
	if errHandled, msg := DeleteAgreement(errorhandler, "agreementId2", db); !errHandled {
		t.Errorf("expected to receive error")
	} else if _, ok := myError.(*NotFoundError); !ok {
		t.Errorf("expected error of type NotFound, but is %T, %v", myError, myError)
	} else if msg != nil {
		t.Errorf("should not have returned a message object, %v", msg)
	}

}

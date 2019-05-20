// +build unit

package text_language

import (
	"testing"

	"github.com/open-horizon/anax/externalpolicy"
)

func Test_Validate_Succeed1(t *testing.T) {
	// "constraints": [
	//	"iame2edev == true && cpu == 3",
	//	"hello == world"
	//  ]
	textCLP := text_language.TextConstraintLanguagePlugin{}
	testConstraints := []externalpolicy.ConstraintExpression{"iame2edev == true && cpu == 3", "hello == world"}
	validated, err := textCLP.validate(testConstraints)

	if !validated {
		t.Errorf("Should validate successfully but not")
	} else if err != nil {
		t.Errorf("Should validated and don't return err, but returned err: %v", err)
	}

}

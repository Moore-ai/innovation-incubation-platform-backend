package agent

import (
	"encoding/json"
	"regexp"
)

var (
	rePhone      = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	reIDCard     = regexp.MustCompile(`\b\d{17}[\dXx]\b`)
	reCreditCode = regexp.MustCompile(`\b[0-9A-HJ-NPQRTUWXY]{18}\b`)
)

func maskSensitive(raw json.RawMessage) json.RawMessage {
	s := string(raw)
	s = rePhone.ReplaceAllString(s, "1**********")
	s = reIDCard.ReplaceAllString(s, "******************")
	s = reCreditCode.ReplaceAllString(s, "******************")
	return json.RawMessage(s)
}

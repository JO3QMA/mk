package email

import "regexp"

// emailRe follows the WHATWG HTML spec pattern that Misskey's UtilityService
// uses for email format validation.
var emailRe = regexp.MustCompile(
	`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`,
)

// ValidateFormat reports whether email matches the WHATWG email format.
func ValidateFormat(email string) bool {
	return emailRe.MatchString(email)
}

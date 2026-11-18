package output

import "context"

// PhoneCodes checks the SMS codes that prove a phone is the person's (the identity module sends and keeps them)
type PhoneCodes interface {
	// On reports SMS codes are switched on (sms.enabled); off, phones are taken on trust as before
	On() bool
	// Check verifies the code sent to phone (20xxxxxxxx) for purpose. A wrong code counts towards the try limit; a right
	// one is not used up (it ends when it expires), so a booking that failed for another reason can be sent again.
	Check(ctx context.Context, phone, purpose, code string) error
}

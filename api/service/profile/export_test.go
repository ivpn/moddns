package profile

import "context"

// ExportVerifyReauthForTest is a test-only shim that exposes the unexported
// verifyReauth method to black-box tests in the profile_test package.
// This file is compiled only during test builds (it lives in package profile,
// not profile_test, so it can access unexported symbols).
func (p *ProfileService) ExportVerifyReauthForTest(
	ctx context.Context,
	accountId string,
	tokenType string,
	currentPassword, reauthToken *string,
) error {
	return p.verifyReauth(ctx, accountId, tokenType, currentPassword, reauthToken)
}

package auth

import "context"

// LogoutUseCase handles logout — the actual session clearing is done in the handler via cookie removal.
type LogoutUseCase struct{}

// NewLogoutUseCase returns a new LogoutUseCase.
func NewLogoutUseCase() *LogoutUseCase {
	return &LogoutUseCase{}
}

// Execute is a no-op; cookie clearing is performed by the handler.
func (uc *LogoutUseCase) Execute(_ context.Context) error {
	return nil
}

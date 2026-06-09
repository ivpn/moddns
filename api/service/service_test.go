package service

import (
	"context"
	"errors"
	"testing"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Service.DeleteAccount must run inner account-deletion before session cleanup
// and propagate errors from either step. Sessions are removed via Store
// (db.Db), not via the SessionServicer interface — the wrapper delegates
// through Service.DeleteSessionsByAccountID which calls s.Store.

func TestServiceDeleteAccount_InnerErrorShortCircuitsSessions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	accountID := "507f1f77bcf86cd799439011"
	req := requests.AccountDeletionRequest{DeletionCode: "DELETE123"}
	mfa := &model.MfaData{}
	innerErr := errors.New("inner failure")

	accountSrv := mocks.NewAccountServicer(t)
	store := mocks.NewDb(t)
	svc := &Service{Store: store, AccountServicer: accountSrv}

	accountSrv.On("DeleteAccount", ctx, accountID, req, mfa).Return(innerErr).Once()

	err := svc.DeleteAccount(ctx, accountID, req, mfa)
	require.ErrorIs(t, err, innerErr)
	store.AssertNotCalled(t, "DeleteSessionsByAccountID", mock.Anything, mock.Anything)
}

func TestServiceDeleteAccount_SuccessDeletesSessions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	accountID := "507f1f77bcf86cd799439011"
	req := requests.AccountDeletionRequest{DeletionCode: "DELETE123"}
	mfa := &model.MfaData{}

	accountSrv := mocks.NewAccountServicer(t)
	store := mocks.NewDb(t)
	svc := &Service{Store: store, AccountServicer: accountSrv}

	accountSrv.On("DeleteAccount", ctx, accountID, req, mfa).Return(nil).Once()
	store.On("DeleteSessionsByAccountID", ctx, accountID).Return(nil).Once()

	err := svc.DeleteAccount(ctx, accountID, req, mfa)
	require.NoError(t, err)
}

func TestServiceDeleteAccount_PropagatesSessionDeleteError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	accountID := "507f1f77bcf86cd799439011"
	req := requests.AccountDeletionRequest{DeletionCode: "DELETE123"}
	mfa := &model.MfaData{}
	sessionErr := errors.New("session cleanup failed")

	accountSrv := mocks.NewAccountServicer(t)
	store := mocks.NewDb(t)
	svc := &Service{Store: store, AccountServicer: accountSrv}

	accountSrv.On("DeleteAccount", ctx, accountID, req, mfa).Return(nil).Once()
	store.On("DeleteSessionsByAccountID", ctx, accountID).Return(sessionErr).Once()

	// Service.DeleteSessionsByAccountID wraps the store error as ErrDeleteSession.
	err := svc.DeleteAccount(ctx, accountID, req, mfa)
	require.ErrorIs(t, err, ErrDeleteSession)
}

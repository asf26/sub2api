package service

import (
	"context"
	"log/slog"
)

func (s *GatewayService) trySubPilotRecommendForGateway(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	platform string,
	hasForcePlatform bool,
) (*AccountSelectionResult, error) {
	if s == nil {
		return nil, nil
	}
	client := newSubPilotClient(s.cfg)
	if !client.enabled() {
		return nil, nil
	}
	requestID := subPilotRequestID(ctx, "")
	rec, err := client.selectAccount(ctx, subPilotSelectRequest{
		RequestID:  requestID,
		Platform:   platform,
		GroupID:    subPilotGroupID(groupID),
		Model:      requestedModel,
		SessionKey: sessionHash,
	})
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, nil
	}
	if _, excluded := excludedIDs[rec.AccountID]; excluded {
		return nil, nil
	}
	accounts, _, err := s.listSchedulableAccounts(ctx, groupID, platform, hasForcePlatform)
	if err != nil {
		return nil, nil
	}
	account := findSubPilotAccount(accounts, rec.AccountID)
	if account == nil || !account.IsSchedulableForModelWithContext(ctx, requestedModel) {
		return nil, nil
	}
	if shouldClearStickySession(account, requestedModel) {
		return nil, nil
	}
	result, err := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
	if err != nil || result == nil || !result.Acquired {
		return nil, nil
	}
	if !s.checkAndRegisterSession(ctx, account, sessionHash) {
		result.ReleaseFunc()
		return nil, nil
	}
	selection, err := s.newSelectionResult(ctx, account, true, result.ReleaseFunc, nil)
	if err != nil {
		result.ReleaseFunc()
		return nil, err
	}
	rememberSubPilotLease(rec.RequestID, account.ID, rec.LeaseID)
	slog.Debug("subpilot selected gateway account", "account_id", account.ID, "group_id", derefGroupID(groupID), "platform", platform)
	return selection, nil
}

func (s *OpenAIGatewayService) trySubPilotRecommendForOpenAI(
	ctx context.Context,
	groupID *int64,
	platform string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requireCompact bool,
	requiredCapability OpenAIEndpointCapability,
) (*AccountSelectionResult, error) {
	if s == nil {
		return nil, nil
	}
	client := newSubPilotClient(s.cfg)
	if !client.enabled() {
		return nil, nil
	}
	requestID := subPilotRequestID(ctx, "")
	rec, err := client.selectAccount(ctx, subPilotSelectRequest{
		RequestID:  requestID,
		Platform:   platform,
		GroupID:    subPilotGroupID(groupID),
		Model:      requestedModel,
		SessionKey: sessionHash,
	})
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, nil
	}
	if _, excluded := excludedIDs[rec.AccountID]; excluded {
		return nil, nil
	}
	account, ok := s.findSubPilotOpenAIAccount(ctx, rec.AccountID, groupID, platform, requestedModel, requireCompact, requiredCapability)
	if !ok {
		return nil, nil
	}
	result, err := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
	if err != nil || result == nil || !result.Acquired {
		return nil, nil
	}
	selection, err := s.newAcquiredSelectionResult(ctx, account, result.ReleaseFunc)
	if err != nil {
		return nil, err
	}
	if sessionHash != "" {
		_ = s.refreshStickySessionTTL(ctx, groupID, sessionHash, openaiStickySessionTTL)
	}
	rememberSubPilotLease(rec.RequestID, account.ID, rec.LeaseID)
	slog.Debug("subpilot selected openai account", "account_id", account.ID, "group_id", derefGroupID(groupID), "platform", platform)
	return selection, nil
}

func (s *OpenAIGatewayService) findSubPilotOpenAIAccount(
	ctx context.Context,
	accountID int64,
	groupID *int64,
	platform string,
	requestedModel string,
	requireCompact bool,
	requiredCapability OpenAIEndpointCapability,
) (*Account, bool) {
	accounts, err := s.listSchedulableAccounts(ctx, groupID, platform)
	if err != nil {
		return nil, false
	}
	account := findSubPilotAccount(accounts, accountID)
	if account == nil {
		return nil, false
	}
	account = s.recheckSelectedOpenAIAccountFromDB(ctx, account, groupID, platform, requestedModel, requireCompact, requiredCapability)
	if account == nil {
		return nil, false
	}
	if groupID != nil && s.needsUpstreamChannelRestrictionCheck(ctx, groupID) && s.isUpstreamModelRestrictedByChannel(ctx, *groupID, account, requestedModel, requireCompact) {
		return nil, false
	}
	return account, true
}

func findSubPilotAccount(accounts []Account, accountID int64) *Account {
	for i := range accounts {
		if accounts[i].ID == accountID {
			return &accounts[i]
		}
	}
	return nil
}

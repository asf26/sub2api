package service

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func (s *GatewayService) trySubPilotRecommendForGateway(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	platform string,
	hasForcePlatform bool,
) (*AccountSelectionResult, bool, error) {
	if s == nil {
		return nil, false, nil
	}
	client := newSubPilotClient(s.cfg)
	if !client.enabled() {
		return nil, false, nil
	}
	requestID := subPilotRequestID(ctx, "")
	localExcluded := cloneSubPilotExcludedIDs(excludedIDs)
	for {
		rec, handled, err := client.selectAccountWithOwnership(ctx, subPilotSelectRequest{
			RequestID:          requestID,
			Platform:           platform,
			GroupID:            subPilotGroupID(groupID),
			Model:              requestedModel,
			SessionKey:         sessionHash,
			ExcludedAccountIDs: subPilotExcludedAccountIDs(localExcluded),
		})
		if err != nil || !handled {
			return nil, handled, err
		}
		if rec == nil {
			return nil, true, ErrNoAvailableAccounts
		}
		if _, excluded := localExcluded[rec.AccountID]; excluded {
			if !rec.LastResort {
				releaseSubPilotRecommendation(client, rec)
				return nil, true, ErrNoAvailableAccounts
			}
		}
		accounts, useMixed, listErr := s.listSchedulableAccounts(ctx, groupID, platform, hasForcePlatform)
		if listErr != nil {
			releaseSubPilotRecommendation(client, rec)
			return nil, true, listErr
		}
		account := findSubPilotAccount(accounts, rec.AccountID)
		loadedDirectly := false
		if account == nil && rec.LastResort && s.accountRepo != nil {
			fresh, getErr := s.accountRepo.GetByID(ctx, rec.AccountID)
			if getErr == nil && fresh != nil {
				account = fresh
				loadedDirectly = true
			}
		}
		if !s.isSubPilotGatewayAccountEligibleForRecommendation(ctx, account, requestedModel, rec.LastResort) ||
			(loadedDirectly && !s.subPilotGatewayAccountMatchesGroup(account, groupID)) ||
			!s.isAccountAllowedForPlatform(account, platform, useMixed) {
			releaseSubPilotRecommendation(client, rec)
			if rec.LastResort {
				return nil, true, ErrNoAvailableAccounts
			}
			localExcluded[rec.AccountID] = struct{}{}
			continue
		}
		result, acquireErr := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
		if acquireErr != nil {
			releaseSubPilotRecommendation(client, rec)
			return nil, true, acquireErr
		}
		if result == nil || !result.Acquired {
			releaseSubPilotRecommendation(client, rec)
			if rec.LastResort {
				return nil, true, ErrNoAvailableAccounts
			}
			localExcluded[rec.AccountID] = struct{}{}
			continue
		}
		if !s.checkAndRegisterSession(ctx, account, sessionHash) {
			result.ReleaseFunc()
			releaseSubPilotRecommendation(client, rec)
			if rec.LastResort {
				return nil, true, ErrNoAvailableAccounts
			}
			localExcluded[rec.AccountID] = struct{}{}
			continue
		}
		selection, selectionErr := s.newSelectionResult(ctx, account, true, result.ReleaseFunc, nil)
		if selectionErr != nil {
			result.ReleaseFunc()
			releaseSubPilotRecommendation(client, rec)
			return nil, true, selectionErr
		}
		rememberSubPilotLease(rec.RequestID, account.ID, rec.LeaseID)
		slog.Debug("subpilot selected gateway account", "account_id", account.ID, "group_id", derefGroupID(groupID), "platform", platform)
		return selection, true, nil
	}
}

func (s *GatewayService) isSubPilotGatewayAccountEligible(ctx context.Context, account *Account, requestedModel string) bool {
	return s.isSubPilotGatewayAccountEligibleForRecommendation(ctx, account, requestedModel, false)
}

func (s *GatewayService) isSubPilotGatewayAccountEligibleForRecommendation(ctx context.Context, account *Account, requestedModel string, lastResort bool) bool {
	if account == nil || !s.isModelSupportedByAccountWithContext(ctx, account, requestedModel) {
		return false
	}
	if lastResort {
		return isSubPilotHardEligibleAccount(account)
	}
	return account.IsSchedulableForModelWithContext(ctx, requestedModel) && !shouldClearStickySession(account, requestedModel)
}

func (s *GatewayService) subPilotGatewayAccountMatchesGroup(account *Account, groupID *int64) bool {
	if s != nil && s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		return account != nil
	}
	return s.isAccountInGroup(account, groupID)
}

func isSubPilotHardEligibleAccount(account *Account) bool {
	if account == nil || !account.IsActive() || !account.Schedulable {
		return false
	}
	return !account.AutoPauseOnExpired || account.ExpiresAt == nil || time.Now().Before(*account.ExpiresAt)
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
	requiredTransport OpenAIUpstreamTransport,
	requiredImageCapability OpenAIImagesCapability,
) (*AccountSelectionResult, bool, error) {
	if s == nil {
		return nil, false, nil
	}
	client := newSubPilotClient(s.cfg)
	if !client.enabled() {
		return nil, false, nil
	}
	requestID := subPilotRequestID(ctx, "")
	localExcluded := cloneSubPilotExcludedIDs(excludedIDs)
	for {
		rec, handled, err := client.selectAccountWithOwnership(ctx, subPilotSelectRequest{
			RequestID:          requestID,
			Platform:           platform,
			GroupID:            subPilotGroupID(groupID),
			Model:              requestedModel,
			SessionKey:         sessionHash,
			ExcludedAccountIDs: subPilotExcludedAccountIDs(localExcluded),
		})
		if err != nil || !handled {
			return nil, handled, err
		}
		if rec == nil {
			return nil, true, ErrNoAvailableAccounts
		}
		if _, excluded := localExcluded[rec.AccountID]; excluded {
			if !rec.LastResort {
				releaseSubPilotRecommendation(client, rec)
				return nil, true, ErrNoAvailableAccounts
			}
		}
		account, ok := s.findSubPilotOpenAIAccount(ctx, rec.AccountID, groupID, platform, requestedModel, requireCompact, requiredCapability, requiredTransport, requiredImageCapability, rec.LastResort)
		if !ok {
			releaseSubPilotRecommendation(client, rec)
			if rec.LastResort {
				return nil, true, ErrNoAvailableAccounts
			}
			localExcluded[rec.AccountID] = struct{}{}
			continue
		}
		result, acquireErr := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
		if acquireErr != nil {
			releaseSubPilotRecommendation(client, rec)
			return nil, true, acquireErr
		}
		if result == nil || !result.Acquired {
			releaseSubPilotRecommendation(client, rec)
			if rec.LastResort {
				return nil, true, ErrNoAvailableAccounts
			}
			localExcluded[rec.AccountID] = struct{}{}
			continue
		}
		selection, selectionErr := s.newAcquiredSelectionResult(ctx, account, result.ReleaseFunc)
		if selectionErr != nil {
			result.ReleaseFunc()
			releaseSubPilotRecommendation(client, rec)
			return nil, true, selectionErr
		}
		if !rec.LastResort && sessionHash != "" {
			_ = s.refreshStickySessionTTL(ctx, groupID, sessionHash, openaiStickySessionTTL)
		}
		rememberSubPilotLease(rec.RequestID, account.ID, rec.LeaseID)
		slog.Debug("subpilot selected openai account", "account_id", account.ID, "group_id", derefGroupID(groupID), "platform", platform)
		return selection, true, nil
	}
}

func cloneSubPilotExcludedIDs(excludedIDs map[int64]struct{}) map[int64]struct{} {
	cloned := make(map[int64]struct{}, len(excludedIDs))
	for accountID := range excludedIDs {
		cloned[accountID] = struct{}{}
	}
	return cloned
}

func subPilotExcludedAccountIDs(excludedIDs map[int64]struct{}) []string {
	ids := make([]int64, 0, len(excludedIDs))
	for accountID := range excludedIDs {
		if accountID > 0 {
			ids = append(ids, accountID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	result := make([]string, 0, len(ids))
	for _, accountID := range ids {
		result = append(result, strconv.FormatInt(accountID, 10))
	}
	return result
}

func releaseSubPilotRecommendation(client subPilotClient, rec *subPilotRecommendation) {
	if rec == nil || rec.AccountID <= 0 || rec.LeaseID == "" {
		return
	}
	client.releaseLease(context.Background(), subPilotReleaseLeaseRequest{
		RequestID: rec.RequestID,
		LeaseID:   rec.LeaseID,
		AccountID: strconv.FormatInt(rec.AccountID, 10),
	})
}

func (s *OpenAIGatewayService) findSubPilotOpenAIAccount(
	ctx context.Context,
	accountID int64,
	groupID *int64,
	platform string,
	requestedModel string,
	requireCompact bool,
	requiredCapability OpenAIEndpointCapability,
	requiredTransport OpenAIUpstreamTransport,
	requiredImageCapability OpenAIImagesCapability,
	lastResort bool,
) (*Account, bool) {
	accounts, err := s.listSchedulableAccounts(ctx, groupID, platform)
	if err != nil {
		return nil, false
	}
	account := findSubPilotAccount(accounts, accountID)
	loadedDirectly := false
	if account == nil && lastResort && s.accountRepo != nil {
		fresh, getErr := s.accountRepo.GetByID(ctx, accountID)
		if getErr == nil && fresh != nil {
			account = fresh
			loadedDirectly = true
		}
	}
	if account == nil {
		return nil, false
	}
	if lastResort {
		platform = normalizeOpenAICompatiblePlatform(platform)
		if !isSubPilotHardEligibleAccount(account) || account.Platform != platform || !account.IsOpenAICompatible() {
			return nil, false
		}
		if loadedDirectly && !s.openAIAccountMatchesSchedulingGroup(account, groupID) {
			return nil, false
		}
		if requestedModel != "" && !account.IsModelSupported(requestedModel) {
			return nil, false
		}
		if requireCompact && (!account.IsOpenAI() || openAICompactSupportTier(account) == 0) {
			return nil, false
		}
		if !s.isOpenAIAccountTransportCompatible(account, requiredTransport) || !accountSupportsOpenAICapabilities(account, requiredCapability, requiredImageCapability) {
			return nil, false
		}
		if groupID != nil && s.needsUpstreamChannelRestrictionCheck(ctx, groupID) && s.isUpstreamModelRestrictedByChannel(ctx, *groupID, account, requestedModel, requireCompact) {
			return nil, false
		}
		return account, true
	}
	if !s.isOpenAIAccountTransportCompatible(account, requiredTransport) || !accountSupportsOpenAICapabilities(account, requiredCapability, requiredImageCapability) {
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

package service

import (
	"context"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

func (s *GatewayService) reportSubPilotSuccess(ctx context.Context, usageLog *UsageLog, account *Account) {
	if s == nil || usageLog == nil || account == nil {
		return
	}
	client := newSubPilotClient(s.cfg)
	if !client.enabled() {
		return
	}
	stream := usageLog.Stream
	client.reportSuccess(detachedSubPilotReportContext(ctx), subPilotReportSuccessRequest{
		RequestID:       usageLog.RequestID,
		LeaseID:         takeSubPilotLease(usageLog.RequestID, usageLog.AccountID),
		APIKeyID:        subPilotAPIKeyID(usageLog.APIKeyID),
		AccountID:       strconv.FormatInt(usageLog.AccountID, 10),
		Platform:        subPilotPlatformForAccount(account, ""),
		GroupID:         subPilotUsageGroupID(usageLog),
		Model:           usageLog.Model,
		LatencyMS:       subPilotDurationMS(usageLog.DurationMs),
		FirstTokenMS:    subPilotFirstTokenMS(usageLog.FirstTokenMs),
		RequestType:     subPilotRequestType(usageLog.Stream),
		Stream:          &stream,
		OfficialUSDUsed: usageLog.TotalCost,
	})
}

func (s *OpenAIGatewayService) reportSubPilotSuccess(ctx context.Context, usageLog *UsageLog, account *Account) {
	if s == nil || usageLog == nil || account == nil {
		return
	}
	client := newSubPilotClient(s.cfg)
	if !client.enabled() {
		return
	}
	stream := usageLog.Stream
	client.reportSuccess(detachedSubPilotReportContext(ctx), subPilotReportSuccessRequest{
		RequestID:       usageLog.RequestID,
		LeaseID:         takeSubPilotLease(usageLog.RequestID, usageLog.AccountID),
		APIKeyID:        subPilotAPIKeyID(usageLog.APIKeyID),
		AccountID:       strconv.FormatInt(usageLog.AccountID, 10),
		Platform:        subPilotPlatformForAccount(account, PlatformOpenAI),
		GroupID:         subPilotUsageGroupID(usageLog),
		Model:           usageLog.Model,
		LatencyMS:       subPilotDurationMS(usageLog.DurationMs),
		FirstTokenMS:    subPilotFirstTokenMS(usageLog.FirstTokenMs),
		RequestType:     subPilotRequestType(usageLog.Stream),
		Stream:          &stream,
		OfficialUSDUsed: usageLog.TotalCost,
	})
}

func (s *OpenAIGatewayService) reportSubPilotFailureForOpenAI(accountID int64) {
	if s == nil || accountID <= 0 {
		return
	}
	client := newSubPilotClient(s.cfg)
	if !client.enabled() {
		return
	}
	accountIDText := strconv.FormatInt(accountID, 10)
	platform := PlatformOpenAI
	groupID := ""
	if s.accountRepo != nil {
		if account, err := s.accountRepo.GetByID(context.Background(), accountID); err == nil && account != nil {
			platform = subPilotPlatformForAccount(account, PlatformOpenAI)
			for _, group := range account.AccountGroups {
				if group.GroupID > 0 {
					groupID = strconv.FormatInt(group.GroupID, 10)
					break
				}
			}
		}
	}
	requestID := "schedule-failure-" + accountIDText + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	client.reportFailure(detachedSubPilotReportContext(context.Background()), subPilotReportFailureRequest{
		RequestID:    requestID,
		LeaseID:      syntheticSubPilotLease(requestID, accountID),
		AccountID:    accountIDText,
		Platform:     platform,
		GroupID:      groupID,
		ErrorMessage: "openai schedule result failed",
	})
}

func detachedSubPilotReportContext(parent context.Context) context.Context {
	ctx := context.Background()
	if parent == nil {
		return ctx
	}
	if clientRequestID, ok := parent.Value(ctxkey.ClientRequestID).(string); ok && clientRequestID != "" {
		ctx = context.WithValue(ctx, ctxkey.ClientRequestID, clientRequestID)
	}
	if requestID, ok := parent.Value(ctxkey.RequestID).(string); ok && requestID != "" {
		ctx = context.WithValue(ctx, ctxkey.RequestID, requestID)
	}
	return ctx
}

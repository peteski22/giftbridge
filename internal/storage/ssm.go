package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// SSMAPI defines the SSM operations used by the state store.
type SSMAPI interface {
	// GetParameter retrieves a parameter from SSM.
	GetParameter(
		ctx context.Context,
		params *ssm.GetParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.GetParameterOutput, error)

	// PutParameter stores a parameter in SSM.
	PutParameter(
		ctx context.Context,
		params *ssm.PutParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.PutParameterOutput, error)
}

// StateStore manages sync state in AWS SSM Parameter Store.
type StateStore struct {
	// client is the SSM API client.
	client SSMAPI

	// lastSyncParameterName is the SSM parameter name for last sync time.
	lastSyncParameterName string

	// pendingParameterName is the SSM parameter name for pending donation IDs.
	pendingParameterName string
}

// LastSyncTime returns the timestamp of the last successful sync.
func (s *StateStore) LastSyncTime(ctx context.Context) (time.Time, error) {
	output, err := s.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(s.lastSyncParameterName),
	})
	if err != nil {
		// Parameter not found is not an error - return zero time.
		var notFoundErr *types.ParameterNotFound
		if errors.As(err, &notFoundErr) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("getting parameter from SSM: %w", err)
	}

	if output.Parameter == nil || output.Parameter.Value == nil {
		return time.Time{}, nil
	}

	t, err := time.Parse(time.RFC3339, *output.Parameter.Value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing time from parameter: %w", err)
	}

	return t, nil
}

// SetLastSyncTime updates the last sync timestamp.
func (s *StateStore) SetLastSyncTime(ctx context.Context, t time.Time) error {
	_, err := s.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(s.lastSyncParameterName),
		Overwrite: aws.Bool(true),
		Type:      types.ParameterTypeString,
		Value:     aws.String(t.Format(time.RFC3339)),
	})
	if err != nil {
		return fmt.Errorf("putting parameter to SSM: %w", err)
	}

	return nil
}

// PendingDonationIDs returns the list of donation IDs still to be processed.
func (s *StateStore) PendingDonationIDs(ctx context.Context) ([]string, error) {
	output, err := s.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(s.pendingParameterName),
	})
	if err != nil {
		var notFoundErr *types.ParameterNotFound
		if errors.As(err, &notFoundErr) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting pending donations from SSM: %w", err)
	}

	if output.Parameter == nil || output.Parameter.Value == nil {
		return nil, nil
	}

	value := *output.Parameter.Value
	if value == "" {
		return nil, nil
	}

	// Store as comma-separated for efficiency (saves ~4 bytes per ID vs JSON).
	return strings.Split(value, ","), nil
}

// SetPendingDonationIDs stores the list of donation IDs to be processed.
func (s *StateStore) SetPendingDonationIDs(ctx context.Context, ids []string) error {
	value := strings.Join(ids, ",")

	_, err := s.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(s.pendingParameterName),
		Overwrite: aws.Bool(true),
		Type:      types.ParameterTypeString,
		Value:     aws.String(value),
	})
	if err != nil {
		return fmt.Errorf("putting pending donations to SSM: %w", err)
	}

	return nil
}

// RemovePendingDonationID removes a single ID from the pending list after processing.
func (s *StateStore) RemovePendingDonationID(ctx context.Context, id string) error {
	ids, err := s.PendingDonationIDs(ctx)
	if err != nil {
		return fmt.Errorf("getting pending IDs: %w", err)
	}

	// Filter out the processed ID.
	remaining := make([]string, 0, len(ids))
	for _, existingID := range ids {
		if existingID != id {
			remaining = append(remaining, existingID)
		}
	}

	return s.SetPendingDonationIDs(ctx, remaining)
}

// StateStoreOption configures a StateStore.
type StateStoreOption func(*StateStore)

// WithPendingParameter sets the SSM parameter name for pending donation IDs.
func WithPendingParameter(name string) StateStoreOption {
	return func(s *StateStore) {
		s.pendingParameterName = name
	}
}

// NewStateStore creates a new SSM-backed state store.
func NewStateStore(client SSMAPI, lastSyncParameterName string, opts ...StateStoreOption) (*StateStore, error) {
	if client == nil {
		return nil, errors.New("ssm client is required")
	}
	if lastSyncParameterName == "" {
		return nil, errors.New("parameter name is required")
	}

	store := &StateStore{
		client:                client,
		lastSyncParameterName: lastSyncParameterName,
	}

	for _, opt := range opts {
		opt(store)
	}

	// Default pending parameter name if not set.
	if store.pendingParameterName == "" {
		// Derive from the sync time parameter by replacing the suffix.
		store.pendingParameterName = lastSyncParameterName[:len(lastSyncParameterName)-len("last-sync-time")] + "pending-donations"
	}

	return store, nil
}

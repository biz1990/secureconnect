package cockroach

import (
	"context"
	"fmt"
	"time"
	
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	
	"secureconnect-backend/internal/domain"
)

// CallRepository handles call data operations
type CallRepository struct {
	pool *pgxpool.Pool
}

// NewCallRepository creates a new call repository
func NewCallRepository(pool *pgxpool.Pool) *CallRepository {
	return &CallRepository{pool: pool}
}

// Create creates a new call record
func (r *CallRepository) Create(ctx context.Context, call *domain.Call) error {
	query := `
		INSERT INTO calls (
			call_id, conversation_id, caller_id, call_type, status, started_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`
	
	_, err := r.pool.Exec(ctx, query,
		call.CallID,
		call.ConversationID,
		call.CallerID,
		call.CallType,
		call.Status,
		call.StartedAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create call: %w", err)
	}
	
	return nil
}

// UpdateStatus updates call status
func (r *CallRepository) UpdateStatus(ctx context.Context, callID uuid.UUID, status string) error {
	query := `
		UPDATE calls
		SET status = $2
		WHERE call_id = $1
	`
	
	_, err := r.pool.Exec(ctx, query, callID, status)
	if err != nil {
		return fmt.Errorf("failed to update call status: %w", err)
	}
	
	return nil
}

// EndCall marks a call as ended and calculates duration
func (r *CallRepository) EndCall(ctx context.Context, callID uuid.UUID) error {
	query := `
		UPDATE calls
		SET status = 'ended',
		    ended_at = NOW(),
		    duration = EXTRACT(EPOCH FROM (NOW() - started_at))::INT
		WHERE call_id = $1
	`
	
	_, err := r.pool.Exec(ctx, query, callID)
	if err != nil {
		return fmt.Errorf("failed to end call: %w", err)
	}
	
	return nil
}

// GetByID retrieves a call by ID
func (r *CallRepository) GetByID(ctx context.Context, callID uuid.UUID) (*domain.Call, error) {
	query := `
		SELECT call_id, conversation_id, caller_id, call_type, status,
		       started_at, ended_at, duration
		FROM calls
		WHERE call_id = $1
	`
	
	call := &domain.Call{}
	err := r.pool.QueryRow(ctx, query, callID).Scan(
		&call.CallID,
		&call.ConversationID,
		&call.CallerID,
		&call.CallType,
		&call.Status,
		&call.StartedAt,
		&call.EndedAt,
		&call.Duration,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("call not found")
		}
		return nil, fmt.Errorf("failed to get call: %w", err)
	}
	
	return call, nil
}

// GetUserCalls retrieves all calls for a user
func (r *CallRepository) GetUserCalls(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Call, error) {
	query := `
		SELECT c.call_id, c.conversation_id, c.caller_id, c.call_type, c.status,
		       c.started_at, c.ended_at, c.duration
		FROM calls c
		LEFT JOIN call_participants cp ON c.call_id = cp.call_id
		WHERE c.caller_id = $1 OR cp.user_id = $1
		ORDER BY c.started_at DESC
		LIMIT $2 OFFSET $3
	`
	
	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get user calls: %w", err)
	}
	defer rows.Close()
	
	var calls []*domain.Call
	for rows.Next() {
		call := &domain.Call{}
		err := rows.Scan(
			&call.CallID,
			&call.ConversationID,
			&call.CallerID,
			&call.CallType,
			&call.Status,
			&call.StartedAt,
			&call.EndedAt,
			&call.Duration,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan call: %w", err)
		}
		calls = append(calls, call)
	}
	
	return calls, nil
}

// AddParticipant adds a participant to a call
func (r *CallRepository) AddParticipant(ctx context.Context, callID, userID uuid.UUID) error {
	query := `
		INSERT INTO call_participants (call_id, user_id, joined_at, is_muted, is_video_on)
		VALUES ($1, $2, $3, false, true)
	`
	
	_, err := r.pool.Exec(ctx, query, callID, userID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add participant: %w", err)
	}
	
	return nil
}

// RemoveParticipant removes a participant from a call
func (r *CallRepository) RemoveParticipant(ctx context.Context, callID, userID uuid.UUID) error {
	query := `
		UPDATE call_participants
		SET left_at = $3
		WHERE call_id = $1 AND user_id = $2
	`
	
	_, err := r.pool.Exec(ctx, query, callID, userID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to remove participant: %w", err)
	}
	
	return nil
}

// GetParticipants retrieves all participants in a call
func (r *CallRepository) GetParticipants(ctx context.Context, callID uuid.UUID) ([]*domain.CallParticipant, error) {
	query := `
		SELECT call_id, user_id, joined_at, left_at, is_muted, is_video_on
		FROM call_participants
		WHERE call_id = $1
		ORDER BY joined_at ASC
	`
	
	rows, err := r.pool.Query(ctx, query, callID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}
	defer rows.Close()
	
	var participants []*domain.CallParticipant
	for rows.Next() {
		p := &domain.CallParticipant{}
		err := rows.Scan(
			&p.CallID,
			&p.UserID,
			&p.JoinedAt,
			&p.LeftAt,
			&p.IsMuted,
			&p.IsVideoOn,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan participant: %w", err)
		}
		participants = append(participants, p)
	}
	
	return participants, nil
}

// UpdateParticipantMedia updates participant's media state
func (r *CallRepository) UpdateParticipantMedia(ctx context.Context, callID, userID uuid.UUID, isMuted, isVideoOn bool) error {
	query := `
		UPDATE call_participants
		SET is_muted = $3, is_video_on = $4
		WHERE call_id = $1 AND user_id = $2
	`
	
	_, err := r.pool.Exec(ctx, query, callID, userID, isMuted, isVideoOn)
	if err != nil {
		return fmt.Errorf("failed to update participant media: %w", err)
	}
	
	return nil
}

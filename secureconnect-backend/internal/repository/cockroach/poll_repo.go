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

// PollRepository handles poll data operations in CockroachDB
type PollRepository struct {
	pool *pgxpool.Pool
}

// NewPollRepository creates a new PollRepository
func NewPollRepository(pool *pgxpool.Pool) *PollRepository {
	return &PollRepository{pool: pool}
}

// CreatePoll creates a new poll with its options in a transaction
func (r *PollRepository) CreatePoll(ctx context.Context, poll *domain.Poll, options []string) error {
	// Begin transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert poll
	query := `
		INSERT INTO polls (poll_id, conversation_id, creator_id, question, poll_type, allow_vote_change, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`

	err = tx.QueryRow(ctx, query,
		poll.PollID,
		poll.ConversationID,
		poll.CreatorID,
		poll.Question,
		poll.PollType,
		poll.AllowVoteChange,
		poll.ExpiresAt,
	).Scan(&poll.CreatedAt, &poll.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create poll: %w", err)
	}

	// Insert poll options
	for i, optionText := range options {
		optionQuery := `
			INSERT INTO poll_options (option_id, poll_id, option_text, display_order)
			VALUES ($1, $2, $3, $4)
			RETURNING created_at
		`

		optionID := uuid.New()
		var createdAt time.Time
		err = tx.QueryRow(ctx, optionQuery, optionID, poll.PollID, optionText, i).Scan(&createdAt)
		if err != nil {
			return fmt.Errorf("failed to create poll option: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetPollByID retrieves a poll by ID
func (r *PollRepository) GetPollByID(ctx context.Context, pollID uuid.UUID) (*domain.Poll, error) {
	query := `
		SELECT poll_id, conversation_id, creator_id, question, poll_type, allow_vote_change, 
		       expires_at, is_closed, closed_at, created_at, updated_at
		FROM polls
		WHERE poll_id = $1
	`

	poll := &domain.Poll{}
	err := r.pool.QueryRow(ctx, query, pollID).Scan(
		&poll.PollID,
		&poll.ConversationID,
		&poll.CreatorID,
		&poll.Question,
		&poll.PollType,
		&poll.AllowVoteChange,
		&poll.ExpiresAt,
		&poll.IsClosed,
		&poll.ClosedAt,
		&poll.CreatedAt,
		&poll.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	return poll, nil
}

// GetPollByIDWithVotes retrieves a poll by ID with vote counts
func (r *PollRepository) GetPollByIDWithVotes(ctx context.Context, pollID uuid.UUID) (*domain.Poll, error) {
	query := `
		SELECT p.poll_id, p.conversation_id, p.creator_id, p.question, p.poll_type, p.allow_vote_change,
		       p.expires_at, p.is_closed, p.closed_at, p.created_at, p.updated_at,
		       COALESCE(COUNT(DISTINCT v.vote_id), 0) as total_votes,
		       COALESCE(COUNT(DISTINCT v.user_id), 0) as total_voters
		FROM polls p
		LEFT JOIN poll_votes v ON p.poll_id = v.poll_id
		WHERE p.poll_id = $1
		GROUP BY p.poll_id, p.conversation_id, p.creator_id, p.question, p.poll_type, p.allow_vote_change,
		         p.expires_at, p.is_closed, p.closed_at, p.created_at, p.updated_at
	`

	poll := &domain.Poll{}
	err := r.pool.QueryRow(ctx, query, pollID).Scan(
		&poll.PollID,
		&poll.ConversationID,
		&poll.CreatorID,
		&poll.Question,
		&poll.PollType,
		&poll.AllowVoteChange,
		&poll.ExpiresAt,
		&poll.IsClosed,
		&poll.ClosedAt,
		&poll.CreatedAt,
		&poll.UpdatedAt,
		&poll.TotalVotes,
		&poll.TotalVoters,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll with votes: %w", err)
	}

	return poll, nil
}

// GetPollByIDWithUserVote retrieves a poll by ID with user's vote information
func (r *PollRepository) GetPollByIDWithUserVote(ctx context.Context, pollID, userID uuid.UUID) (*domain.Poll, error) {
	query := `
		SELECT p.poll_id, p.conversation_id, p.creator_id, p.question, p.poll_type, p.allow_vote_change,
		       p.expires_at, p.is_closed, p.closed_at, p.created_at, p.updated_at,
		       COALESCE(COUNT(DISTINCT v.vote_id), 0) as total_votes,
		       COALESCE(COUNT(DISTINCT v.user_id), 0) as total_voters,
		       EXISTS(SELECT 1 FROM poll_votes WHERE poll_id = p.poll_id AND user_id = $2) as user_voted
		FROM polls p
		LEFT JOIN poll_votes v ON p.poll_id = v.poll_id
		WHERE p.poll_id = $1
		GROUP BY p.poll_id, p.conversation_id, p.creator_id, p.question, p.poll_type, p.allow_vote_change,
		         p.expires_at, p.is_closed, p.closed_at, p.created_at, p.updated_at
	`

	poll := &domain.Poll{}
	err := r.pool.QueryRow(ctx, query, pollID, userID).Scan(
		&poll.PollID,
		&poll.ConversationID,
		&poll.CreatorID,
		&poll.Question,
		&poll.PollType,
		&poll.AllowVoteChange,
		&poll.ExpiresAt,
		&poll.IsClosed,
		&poll.ClosedAt,
		&poll.CreatedAt,
		&poll.UpdatedAt,
		&poll.TotalVotes,
		&poll.TotalVoters,
		&poll.UserVoted,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll with user vote: %w", err)
	}

	// If user has voted, get their vote options
	if poll.UserVoted {
		voteOptionsQuery := `
			SELECT option_id
			FROM poll_votes
			WHERE poll_id = $1 AND user_id = $2
		`

		rows, err := r.pool.Query(ctx, voteOptionsQuery, pollID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user vote options: %w", err)
		}
		defer rows.Close()

		var optionIDs []uuid.UUID
		for rows.Next() {
			var optionID uuid.UUID
			if err := rows.Scan(&optionID); err != nil {
				return nil, fmt.Errorf("failed to scan option ID: %w", err)
			}
			optionIDs = append(optionIDs, optionID)
		}

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating vote options: %w", err)
		}

		poll.UserVoteOptions = optionIDs
	}

	return poll, nil
}

// GetPollsByConversation retrieves polls for a conversation with pagination
func (r *PollRepository) GetPollsByConversation(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*domain.Poll, int, error) {
	// Get total count
	countQuery := `
		SELECT COUNT(*)
		FROM polls
		WHERE conversation_id = $1
	`

	var total int
	err := r.pool.QueryRow(ctx, countQuery, conversationID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count polls: %w", err)
	}

	// Get polls
	query := `
		SELECT poll_id, conversation_id, creator_id, question, poll_type, allow_vote_change,
		       expires_at, is_closed, closed_at, created_at, updated_at
		FROM polls
		WHERE conversation_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, conversationID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get polls: %w", err)
	}
	defer rows.Close()

	polls := make([]*domain.Poll, 0)
	for rows.Next() {
		poll := &domain.Poll{}
		err := rows.Scan(
			&poll.PollID,
			&poll.ConversationID,
			&poll.CreatorID,
			&poll.Question,
			&poll.PollType,
			&poll.AllowVoteChange,
			&poll.ExpiresAt,
			&poll.IsClosed,
			&poll.ClosedAt,
			&poll.CreatedAt,
			&poll.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan poll: %w", err)
		}
		polls = append(polls, poll)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating polls: %w", err)
	}

	return polls, total, nil
}

// GetPollOptions retrieves options for a poll
func (r *PollRepository) GetPollOptions(ctx context.Context, pollID uuid.UUID) ([]*domain.PollOption, error) {
	query := `
		SELECT option_id, poll_id, option_text, display_order, created_at
		FROM poll_options
		WHERE poll_id = $1
		ORDER BY display_order ASC
	`

	rows, err := r.pool.Query(ctx, query, pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll options: %w", err)
	}
	defer rows.Close()

	options := make([]*domain.PollOption, 0)
	for rows.Next() {
		option := &domain.PollOption{}
		err := rows.Scan(
			&option.OptionID,
			&option.PollID,
			&option.OptionText,
			&option.DisplayOrder,
			&option.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan poll option: %w", err)
		}
		options = append(options, option)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating poll options: %w", err)
	}

	return options, nil
}

// GetPollOptionsWithVotes retrieves options for a poll with vote counts
func (r *PollRepository) GetPollOptionsWithVotes(ctx context.Context, pollID uuid.UUID) ([]*domain.PollOption, error) {
	query := `
		SELECT po.option_id, po.poll_id, po.option_text, po.display_order, po.created_at,
		       COALESCE(COUNT(pv.vote_id), 0) as vote_count,
		       CASE 
		         WHEN (SELECT COUNT(*) FROM poll_votes WHERE poll_id = po.poll_id) > 0
		         THEN ROUND((COUNT(pv.vote_id)::NUMERIC / (SELECT COUNT(*)::NUMERIC FROM poll_votes WHERE poll_id = po.poll_id)) * 100, 2)
		         ELSE 0 
		       END as vote_percentage
		FROM poll_options po
		LEFT JOIN poll_votes pv ON po.option_id = pv.option_id
		WHERE po.poll_id = $1
		GROUP BY po.option_id, po.poll_id, po.option_text, po.display_order, po.created_at
		ORDER BY po.display_order ASC
	`

	rows, err := r.pool.Query(ctx, query, pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll options with votes: %w", err)
	}
	defer rows.Close()

	options := make([]*domain.PollOption, 0)
	for rows.Next() {
		option := &domain.PollOption{}
		err := rows.Scan(
			&option.OptionID,
			&option.PollID,
			&option.OptionText,
			&option.DisplayOrder,
			&option.CreatedAt,
			&option.VoteCount,
			&option.VotePercent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan poll option with votes: %w", err)
		}
		options = append(options, option)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating poll options with votes: %w", err)
	}

	return options, nil
}

// CastVote casts a vote in a poll (transactional)
func (r *PollRepository) CastVote(ctx context.Context, vote *domain.PollVote) error {
	// Begin transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert vote
	query := `
		INSERT INTO poll_votes (vote_id, poll_id, option_id, user_id, voted_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (poll_id, user_id, option_id) DO NOTHING
		RETURNING voted_at
	`

	err = tx.QueryRow(ctx, query,
		vote.VoteID,
		vote.PollID,
		vote.OptionID,
		vote.UserID,
		vote.VotedAt,
	).Scan(&vote.VotedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			// Vote already exists (duplicate), but that's okay
			return nil
		}
		return fmt.Errorf("failed to cast vote: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ChangeVote changes a user's vote in a poll (transactional)
func (r *PollRepository) ChangeVote(ctx context.Context, pollID, userID uuid.UUID, newOptionIDs []uuid.UUID) error {
	// Begin transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Delete existing votes for this user in this poll
	deleteQuery := `
		DELETE FROM poll_votes
		WHERE poll_id = $1 AND user_id = $2
	`

	_, err = tx.Exec(ctx, deleteQuery, pollID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete existing votes: %w", err)
	}

	// Insert new votes
	for _, optionID := range newOptionIDs {
		insertQuery := `
			INSERT INTO poll_votes (vote_id, poll_id, option_id, user_id, voted_at)
			VALUES ($1, $2, $3, $4, $5)
		`

		voteID := uuid.New()
		votedAt := time.Now()
		_, err = tx.Exec(ctx, insertQuery, voteID, pollID, optionID, userID, votedAt)
		if err != nil {
			return fmt.Errorf("failed to insert new vote: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetUserVotes retrieves votes for a user in a poll
func (r *PollRepository) GetUserVotes(ctx context.Context, pollID, userID uuid.UUID) ([]*domain.PollVote, error) {
	query := `
		SELECT vote_id, poll_id, option_id, user_id, voted_at
		FROM poll_votes
		WHERE poll_id = $1 AND user_id = $2
		ORDER BY voted_at DESC
	`

	rows, err := r.pool.Query(ctx, query, pollID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user votes: %w", err)
	}
	defer rows.Close()

	votes := make([]*domain.PollVote, 0)
	for rows.Next() {
		vote := &domain.PollVote{}
		err := rows.Scan(
			&vote.VoteID,
			&vote.PollID,
			&vote.OptionID,
			&vote.UserID,
			&vote.VotedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan vote: %w", err)
		}
		votes = append(votes, vote)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating votes: %w", err)
	}

	return votes, nil
}

// ClosePoll closes a poll
func (r *PollRepository) ClosePoll(ctx context.Context, pollID uuid.UUID) error {
	query := `
		UPDATE polls
		SET is_closed = TRUE, closed_at = NOW(), updated_at = NOW()
		WHERE poll_id = $1
		RETURNING poll_id
	`

	var returnedID uuid.UUID
	err := r.pool.QueryRow(ctx, query, pollID).Scan(&returnedID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("poll not found")
		}
		return fmt.Errorf("failed to close poll: %w", err)
	}

	return nil
}

// DeletePoll deletes a poll (cascade will handle options and votes)
func (r *PollRepository) DeletePoll(ctx context.Context, pollID uuid.UUID) error {
	query := `DELETE FROM polls WHERE poll_id = $1 RETURNING poll_id`

	var returnedID uuid.UUID
	err := r.pool.QueryRow(ctx, query, pollID).Scan(&returnedID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("poll not found")
		}
		return fmt.Errorf("failed to delete poll: %w", err)
	}

	return nil
}

// GetActivePolls retrieves active (not closed and not expired) polls
func (r *PollRepository) GetActivePolls(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*domain.Poll, int, error) {
	// Get total count
	countQuery := `
		SELECT COUNT(*)
		FROM polls
		WHERE conversation_id = $1 AND is_closed = FALSE AND (expires_at IS NULL OR expires_at > NOW())
	`

	var total int
	err := r.pool.QueryRow(ctx, countQuery, conversationID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count active polls: %w", err)
	}

	// Get active polls
	query := `
		SELECT poll_id, conversation_id, creator_id, question, poll_type, allow_vote_change,
		       expires_at, is_closed, closed_at, created_at, updated_at
		FROM polls
		WHERE conversation_id = $1 AND is_closed = FALSE AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, conversationID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get active polls: %w", err)
	}
	defer rows.Close()

	polls := make([]*domain.Poll, 0)
	for rows.Next() {
		poll := &domain.Poll{}
		err := rows.Scan(
			&poll.PollID,
			&poll.ConversationID,
			&poll.CreatorID,
			&poll.Question,
			&poll.PollType,
			&poll.AllowVoteChange,
			&poll.ExpiresAt,
			&poll.IsClosed,
			&poll.ClosedAt,
			&poll.CreatedAt,
			&poll.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan poll: %w", err)
		}
		polls = append(polls, poll)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating polls: %w", err)
	}

	return polls, total, nil
}

// IsPollCreator checks if a user is the creator of a poll
func (r *PollRepository) IsPollCreator(ctx context.Context, pollID, userID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(SELECT 1 FROM polls WHERE poll_id = $1 AND creator_id = $2)
	`

	var exists bool
	err := r.pool.QueryRow(ctx, query, pollID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check poll creator: %w", err)
	}

	return exists, nil
}

// GetPollsByCreator retrieves polls created by a user
func (r *PollRepository) GetPollsByCreator(ctx context.Context, creatorID uuid.UUID, limit, offset int) ([]*domain.Poll, int, error) {
	// Get total count
	countQuery := `
		SELECT COUNT(*)
		FROM polls
		WHERE creator_id = $1
	`

	var total int
	err := r.pool.QueryRow(ctx, countQuery, creatorID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count polls by creator: %w", err)
	}

	// Get polls
	query := `
		SELECT poll_id, conversation_id, creator_id, question, poll_type, allow_vote_change,
		       expires_at, is_closed, closed_at, created_at, updated_at
		FROM polls
		WHERE creator_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, creatorID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get polls by creator: %w", err)
	}
	defer rows.Close()

	polls := make([]*domain.Poll, 0)
	for rows.Next() {
		poll := &domain.Poll{}
		err := rows.Scan(
			&poll.PollID,
			&poll.ConversationID,
			&poll.CreatorID,
			&poll.Question,
			&poll.PollType,
			&poll.AllowVoteChange,
			&poll.ExpiresAt,
			&poll.IsClosed,
			&poll.ClosedAt,
			&poll.CreatedAt,
			&poll.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan poll: %w", err)
		}
		polls = append(polls, poll)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating polls: %w", err)
	}

	return polls, total, nil
}

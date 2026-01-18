# Vote/Poll Feature Implementation Summary

## Overview
This document provides a complete summary of the Vote/Poll feature implementation for the SecureConnect backend system.

## Implementation Files

### 1. Database Schema
**File:** [`secureconnect-backend/scripts/polls-schema.sql`](secureconnect-backend/scripts/polls-schema.sql)

**Tables:**
- `polls` - Stores poll metadata
  - poll_id (UUID, PK)
  - conversation_id (UUID, FK to conversations)
  - creator_id (UUID, FK to users)
  - question (STRING)
  - poll_type (STRING: 'single' or 'multi')
  - allow_vote_change (BOOLEAN)
  - expires_at (TIMESTAMPTZ, nullable)
  - is_closed (BOOLEAN)
  - closed_at (TIMESTAMPTZ, nullable)
  - created_at (TIMESTAMPTZ)
  - updated_at (TIMESTAMPTZ)

- `poll_options` - Stores poll options
  - option_id (UUID, PK)
  - poll_id (UUID, FK to polls)
  - option_text (STRING)
  - display_order (INT)
  - created_at (TIMESTAMPTZ)

- `poll_votes` - Stores user votes
  - vote_id (UUID, PK)
  - poll_id (UUID, FK to polls)
  - option_id (UUID, FK to poll_options)
  - user_id (UUID, FK to users)
  - voted_at (TIMESTAMPTZ)

**Views:**
- `poll_results_view` - Aggregates poll data with vote counts
- `poll_option_results_view` - Aggregates option data with vote percentages

**Functions:**
- `user_has_voted(poll_uuid, user_uuid)` - Check if user voted
- `get_user_votes(poll_uuid, user_uuid)` - Get user's votes
- `close_poll(poll_uuid)` - Close a poll
- `update_polls_updated_at()` - Auto-update timestamp trigger

### 2. Domain Models
**File:** [`secureconnect-backend/internal/domain/poll.go`](secureconnect-backend/internal/domain/poll.go)

**Types:**
- `Poll` - Poll entity
- `PollCreate` - Poll creation request
- `PollUpdate` - Poll update request
- `PollOption` - Poll option entity
- `PollVote` - Vote entity
- `VoteRequest` - Vote casting request
- `PollResponse` - Poll API response
- `PollListResponse` - Paginated poll list response
- `PollType` - Enum: "single" or "multi"

**Methods:**
- `ToResponse()` - Convert to API response
- `IsExpired()` - Check if poll expired
- `CanVote()` - Check if user can vote
- `CanChangeVote()` - Check if vote can be changed

**Validation Functions:**
- `ValidatePollType()` - Validate poll type
- `ValidateOptions()` - Validate poll options
- `ValidateVoteRequest()` - Validate vote request

**Errors:**
- `ErrInsufficientOptions`
- `ErrTooManyOptions`
- `ErrPollNotFound`
- `ErrPollExpired`
- `ErrPollClosed`
- `ErrAlreadyVoted`
- `ErrMultipleOptionsNotAllowed`
- `ErrAtLeastOneOptionRequired`
- `ErrOptionNotFound`
- `ErrNotPollCreator`
- `ErrInvalidPollType`

### 3. Repository Layer
**File:** [`secureconnect-backend/internal/repository/cockroach/poll_repo.go`](secureconnect-backend/internal/repository/cockroach/poll_repo.go)

**Methods:**
- `CreatePoll(ctx, poll, options)` - Create poll with options (transactional)
- `GetPollByID(ctx, pollID)` - Get poll by ID
- `GetPollByIDWithVotes(ctx, pollID)` - Get poll with vote counts
- `GetPollByIDWithUserVote(ctx, pollID, userID)` - Get poll with user's vote info
- `GetPollsByConversation(ctx, conversationID, limit, offset)` - List polls for conversation
- `GetPollOptions(ctx, pollID)` - Get poll options
- `GetPollOptionsWithVotes(ctx, pollID)` - Get options with vote counts
- `CastVote(ctx, vote)` - Cast a vote
- `ChangeVote(ctx, pollID, userID, newOptionIDs)` - Change user's vote (transactional)
- `GetUserVotes(ctx, pollID, userID)` - Get user's votes
- `ClosePoll(ctx, pollID)` - Close a poll
- `DeletePoll(ctx, pollID)` - Delete a poll (cascade)
- `GetActivePolls(ctx, conversationID, limit, offset)` - Get active polls
- `IsPollCreator(ctx, pollID, userID)` - Check if user is poll creator
- `GetPollsByCreator(ctx, creatorID, limit, offset)` - Get polls by creator

### 4. Service Layer
**File:** [`secureconnect-backend/internal/service/poll/service.go`](secureconnect-backend/internal/service/poll/service.go)

**Methods:**
- `CreatePoll(ctx, input)` - Create a new poll
  - Validates poll type and options
  - Verifies user is conversation participant
  - Creates poll with options in transaction
  - Publishes `poll_created` event

- `GetPoll(ctx, input)` - Get a poll with results
  - Returns poll with options and vote counts
  - Includes user's vote information

- `GetPolls(ctx, input)` - List polls for conversation
  - Paginated results
  - Includes user's vote information for each poll

- `Vote(ctx, input)` - Cast or change a vote
  - Validates poll is active and user can vote
  - Supports single-choice and multi-choice polls
  - Supports vote changes if enabled
  - Publishes `poll_voted` event

- `ClosePoll(ctx, input)` - Close a poll
  - Only poll creator can close (unless force=true)
  - Publishes `poll_closed` event

- `DeletePoll(ctx, input)` - Delete a poll
  - Only poll creator can delete
  - Cascade deletes options and votes

- `GetActivePolls(ctx, input)` - Get active polls
  - Filters out closed and expired polls

**WebSocket Events:**
- `poll_created` - Published when a poll is created
- `poll_voted` - Published when a vote is cast/changed
- `poll_closed` - Published when a poll is closed

### 5. HTTP Handler
**File:** [`secureconnect-backend/internal/handler/http/poll/handler.go`](secureconnect-backend/internal/handler/http/poll/handler.go)

**Endpoints:**

| Method | Path | Description |
|---------|-------|-------------|
| POST | `/v1/polls` | Create a new poll |
| GET | `/v1/polls/:poll_id` | Get a poll with results |
| GET | `/v1/polls` | List polls for a conversation |
| POST | `/v1/polls/vote` | Cast a vote |
| POST | `/v1/polls/close` | Close a poll |
| DELETE | `/v1/polls/:poll_id` | Delete a poll |
| GET | `/v1/polls/active` | Get active polls |

**Request/Response Types:**
- `CreatePollRequest` - Poll creation request
- `VoteRequest` - Vote casting request
- `ClosePollRequest` - Poll close request
- `GetPollsQuery` - Poll list query params

**Authorization:**
- All endpoints require authentication (user_id from JWT)
- Poll creation: User must be conversation participant
- Vote: User must be conversation participant
- Close/Delete: User must be poll creator

### 6. WebSocket Handler
**File:** [`secureconnect-backend/internal/handler/ws/poll_handler.go`](secureconnect-backend/internal/handler/ws/poll_handler.go)

**Components:**
- `PollHub` - Manages WebSocket connections for polls
- `PollClient` - Represents a WebSocket client
- `PollMessage` - WebSocket message structure

**Message Types:**
- `poll_created` - New poll created
- `poll_voted` - Vote cast/changed
- `poll_closed` - Poll closed

**Endpoint:**
- `WS /ws/poll?conversation_id=uuid` - Connect to poll updates for a conversation

**Features:**
- Connection limit (configurable via `WS_MAX_POLL_CONNECTIONS`)
- Redis Pub/Sub for real-time updates
- Automatic subscription to poll events
- Connection cleanup on disconnect

### 7. Prometheus Metrics
**File:** [`secureconnect-backend/pkg/metrics/poll_metrics.go`](secureconnect-backend/pkg/metrics/poll_metrics.go)

**Metrics:**

| Metric | Type | Labels | Description |
|---------|-------|---------|-------------|
| `polls_created_total` | Counter | poll_type, conversation_id | Total polls created |
| `polls_closed_total` | Counter | conversation_id | Total polls closed |
| `polls_deleted_total` | Counter | conversation_id | Total polls deleted |
| `votes_cast_total` | Counter | poll_type, conversation_id | Total votes cast |
| `votes_changed_total` | Counter | poll_type, conversation_id | Total votes changed |
| `polls_active_total` | Gauge | conversation_id | Active polls count |
| `polls_expired_total` | Counter | conversation_id | Expired polls count |
| `poll_websocket_connections_active` | Gauge | - | Active WebSocket connections |
| `poll_websocket_messages_total` | Counter | type, direction | WebSocket messages |
| `poll_event_published_total` | Counter | event_type, status | Redis events published |
| `poll_creation_duration_seconds` | Histogram | poll_type | Poll creation latency |
| `poll_vote_duration_seconds` | Histogram | poll_type | Vote casting latency |
| `poll_retrieval_duration_seconds` | Histogram | operation | Poll retrieval latency |

**Authorization Metrics:**
- `poll_create_unauthorized_total`
- `poll_vote_unauthorized_total`
- `poll_close_unauthorized_total`
- `poll_delete_unauthorized_total`

**Error Metrics:**
- `poll_create_error_total`
- `poll_vote_error_total`
- `poll_close_error_total`

### 8. Database Initialization
**File:** [`secureconnect-backend/scripts/cockroach-init.sql`](secureconnect-backend/scripts/cockroach-init.sql)

**Changes:**
- Added DROP statements for poll tables
- Added `polls` table
- Added `poll_options` table
- Added `poll_votes` table
- Added `update_polls_updated_at()` function and trigger

## Business Rules

### Single-Choice Polls
- Users can only select one option
- Changing votes replaces previous vote

### Multi-Choice Polls
- Users can select multiple options
- Changing votes replaces all previous votes

### Vote Change Rules
- Controlled by `allow_vote_change` flag
- If enabled, users can change their votes anytime before poll closes
- If disabled, users can only vote once

### Poll Expiration
- `expires_at` field sets expiration time
- Expired polls cannot receive new votes
- Expired polls are not returned in active polls query

### Poll Closure
- Only poll creator can close polls
- Closed polls cannot receive new votes
- Closing publishes `poll_closed` event

### Authorization
- Create poll: User must be conversation participant
- Vote: User must be conversation participant
- Close/Delete: User must be poll creator
- WebSocket: User must be conversation participant

## Integration Points

### Required Dependencies
1. **CockroachDB** - Database for polls, options, votes
2. **Redis** - Pub/Sub for real-time events
3. **Conversation Service** - Verify conversation membership
4. **User Service** - Get user details (creator name)
5. **Auth Middleware** - JWT authentication

### Event Channels
- `poll:{conversation_id}` - Redis Pub/Sub channel for poll events

## API Examples

### Create Poll
```json
POST /v1/polls
{
  "conversation_id": "uuid",
  "question": "What's your favorite color?",
  "poll_type": "single",
  "allow_vote_change": true,
  "expires_at": "2024-12-31T23:59:59Z",
  "options": ["Red", "Blue", "Green", "Yellow"]
}
```

### Vote
```json
POST /v1/polls/vote
{
  "poll_id": "uuid",
  "option_ids": ["uuid"]  // Single for single-choice, multiple for multi-choice
}
```

### Close Poll
```json
POST /v1/polls/close
{
  "poll_id": "uuid",
  "force": false
}
```

### WebSocket Event (poll_created)
```json
{
  "type": "poll_created",
  "data": {
    "poll_id": "uuid",
    "conversation_id": "uuid",
    "question": "...",
    "poll_type": "single",
    "options": [...]
  },
  "timestamp": "2024-01-01T00:00:00Z"
}
```

## Deployment Steps

1. **Run Database Migration**
   ```bash
   cockroach sql --insecure < secureconnect-backend/scripts/polls-schema.sql
   ```

2. **Or Reinitialize Database**
   ```bash
   cockroach sql --insecure < secureconnect-backend/scripts/cockroach-init.sql
   ```

3. **Register Routes**
   Add poll handler routes to your router:
   ```go
   pollHandler := poll.NewHandler(pollService)
   
   router.POST("/v1/polls", authMiddleware, pollHandler.CreatePoll)
   router.GET("/v1/polls/:poll_id", authMiddleware, pollHandler.GetPoll)
   router.GET("/v1/polls", authMiddleware, pollHandler.GetPolls)
   router.POST("/v1/polls/vote", authMiddleware, pollHandler.Vote)
   router.POST("/v1/polls/close", authMiddleware, pollHandler.ClosePoll)
   router.DELETE("/v1/polls/:poll_id", authMiddleware, pollHandler.DeletePoll)
   router.GET("/v1/polls/active", authMiddleware, pollHandler.GetActivePolls)
   
   // WebSocket
   router.GET("/ws/poll", authMiddleware, func(c *gin.Context) {
       pollHub.ServePollWS(c, conversationRepo)
   })
   ```

4. **Initialize Poll Hub**
   ```go
   pollHub := ws.NewPollHub(redisClient)
   ```

5. **Configure Metrics**
   Metrics are automatically registered when imported.

## Testing Recommendations

1. **Unit Tests**
   - Repository layer tests with mock DB
   - Service layer tests with mock dependencies
   - Handler tests with test server

2. **Integration Tests**
   - End-to-end poll creation and voting
   - WebSocket event delivery
   - Vote change scenarios
   - Poll expiration handling

3. **Load Tests**
   - Concurrent voting on same poll
   - High WebSocket connection count
   - Large number of polls per conversation

## Security Considerations

1. **Authorization** - All operations verify user permissions
2. **CORS** - WebSocket origin validation
3. **Rate Limiting** - Consider adding rate limits for voting
4. **Input Validation** - All inputs validated at handler level
5. **SQL Injection** - Parameterized queries throughout
6. **XSS** - Question and options should be sanitized

## Performance Considerations

1. **Indexes** - All foreign keys and query fields indexed
2. **Pagination** - List operations support pagination
3. **Views** - Complex aggregations use materialized views
4. **Connection Pooling** - Using pgxpool for DB connections
5. **Async Events** - WebSocket events published non-blocking
6. **Connection Limits** - WebSocket connections limited via semaphore

## Future Enhancements

1. **Poll Templates** - Reusable poll templates
2. **Anonymous Polls** - Polls without vote tracking
3. **Poll Results Visibility** - Hide results until poll closes
4. **Poll Comments** - Add comments to polls
5. **Poll Notifications** - Push notifications for new polls
6. **Poll Analytics** - Advanced poll analytics and reporting

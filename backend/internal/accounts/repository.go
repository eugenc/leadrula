package accounts

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/handlerid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

const accountCols = `id, public_id, handler_id, type, name, website, timezone, created_at`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

// LoadPrincipal resolves a user public_id into an auth.Principal.
func (r *Repository) LoadPrincipal(ctx context.Context, userPublicID string) (*auth.Principal, error) {
	const q = `
		SELECT u.id, u.public_id, u.account_id, a.public_id, a.type, u.role, u.is_active
		FROM users u JOIN accounts a ON a.id = u.account_id
		WHERE u.public_id = $1`
	p := &auth.Principal{}
	var active bool
	err := r.pool.QueryRow(ctx, q, userPublicID).Scan(
		&p.UserID, &p.UserPublicID, &p.AccountID, &p.AccountPublicID,
		&p.AccountType, &p.Role, &active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !active {
		return nil, ErrNotFound
	}
	return p, nil
}

type AuthUser struct {
	ID           int64
	PublicID     string
	AccountID    int64
	AccountPubID string
	AccountType  string
	Email        string
	PasswordHash *string
	FullName     string
	Role         string
	IsActive     bool
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (*AuthUser, error) {
	const q = `
		SELECT u.id, u.public_id, u.account_id, a.public_id, a.type,
		       u.email, u.password_hash, u.full_name, u.role, u.is_active
		FROM users u JOIN accounts a ON a.id = u.account_id
		WHERE u.email = $1`
	u := &AuthUser{}
	err := r.pool.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.PublicID, &u.AccountID, &u.AccountPubID, &u.AccountType,
		&u.Email, &u.PasswordHash, &u.FullName, &u.Role, &u.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

func (r *Repository) TouchLogin(ctx context.Context, userID int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, userID)
	return err
}

// BuyerSummary is a publisher-oversight row: a buyer with balance + lead count.
type BuyerSummary struct {
	ID        int64   `json:"id"`
	PublicID  string  `json:"public_id"`
	HandlerID string  `json:"handler_id"`
	Name      string  `json:"name"`
	Balance   float64 `json:"balance"`
	LeadCount int     `json:"lead_count"`
}

func (r *Repository) ListBuyers(ctx context.Context, publisherID int64) ([]BuyerSummary, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, a.public_id, a.handler_id, a.name,
		        COALESCE(b.balance,0)::float8,
		        (SELECT count(*) FROM leads l WHERE l.owner_account_id = a.id)
		 FROM accounts a
		 JOIN partnerships p ON p.buyer_id = a.id AND p.publisher_id = $1 AND p.status = 'active'
		 LEFT JOIN buyer_balances b ON b.buyer_id = a.id
		 ORDER BY a.name`, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BuyerSummary
	for rows.Next() {
		var s BuyerSummary
		if err := rows.Scan(&s.ID, &s.PublicID, &s.HandlerID, &s.Name, &s.Balance, &s.LeadCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// PublisherSummary is a buyer-oversight row: an active publisher with lead count.
type PublisherSummary struct {
	ID                   int64  `json:"id"`
	PublicID             string `json:"public_id"`
	HandlerID            string `json:"handler_id"`
	Name                 string `json:"name"`
	LeadCount            int    `json:"lead_count"`
	CollaborationStatus  string `json:"collaboration_status"`
}

func (r *Repository) ListPublishers(ctx context.Context, buyerID int64) ([]PublisherSummary, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, a.public_id, a.handler_id, a.name,
		        (SELECT count(*) FROM leads l WHERE l.owner_account_id = $1 AND l.publisher_id = a.id),
		        COALESCE(c.status::text, 'none')
		 FROM accounts a
		 JOIN partnerships p ON p.publisher_id = a.id AND p.buyer_id = $1 AND p.status = 'active'
		 LEFT JOIN buyer_collaborations c ON c.publisher_id = a.id AND c.buyer_id = $1
		 ORDER BY a.name`, buyerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PublisherSummary
	for rows.Next() {
		var s PublisherSummary
		if err := rows.Scan(&s.ID, &s.PublicID, &s.HandlerID, &s.Name, &s.LeadCount, &s.CollaborationStatus); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) GetAccount(ctx context.Context, id int64) (*Account, error) {
	const q = `SELECT ` + accountCols + ` FROM accounts WHERE id = $1`
	a := &Account{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&a.ID, &a.PublicID, &a.HandlerID, &a.Type, &a.Name, &a.Website, &a.Timezone, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return a, nil
}

func (r *Repository) GetAccountByHandlerID(ctx context.Context, handlerID string) (*Account, error) {
	const q = `SELECT ` + accountCols + ` FROM accounts WHERE handler_id = $1`
	a := &Account{}
	err := r.pool.QueryRow(ctx, q, handlerID).Scan(
		&a.ID, &a.PublicID, &a.HandlerID, &a.Type, &a.Name, &a.Website, &a.Timezone, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return a, nil
}

func (r *Repository) UpdateBuyer(ctx context.Context, id int64, p UpdateBuyerParams) (*Account, error) {
	const q = `
		UPDATE accounts SET
			name = COALESCE($2, name),
			website = COALESCE($3, website),
			timezone = COALESCE($4, timezone)
		WHERE id = $1 AND type = 'buyer'
		RETURNING ` + accountCols
	a := &Account{}
	err := r.pool.QueryRow(ctx, q, id, p.Name, p.Website, p.Timezone).Scan(
		&a.ID, &a.PublicID, &a.HandlerID, &a.Type, &a.Name, &a.Website, &a.Timezone, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return a, nil
}

func (r *Repository) GetUser(ctx context.Context, id int64) (*User, error) {
	const q = `SELECT id, public_id, account_id, email, full_name, role, is_active, prefs, last_login_at, created_at
		FROM users WHERE id = $1`
	return scanUser(r.pool.QueryRow(ctx, q, id))
}

func (r *Repository) GetUserInAccount(ctx context.Context, accountID, userID int64) (*User, error) {
	const q = `SELECT id, public_id, account_id, email, full_name, role, is_active, prefs, last_login_at, created_at
		FROM users WHERE id = $1 AND account_id = $2`
	return scanUser(r.pool.QueryRow(ctx, q, userID, accountID))
}

func (r *Repository) ListUsers(ctx context.Context, accountID int64) ([]User, error) {
	const q = `SELECT id, public_id, account_id, email, full_name, role, is_active, prefs, last_login_at, created_at
		FROM users WHERE account_id = $1 ORDER BY created_at`
	rows, err := r.pool.Query(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

func (r *Repository) UpdateUser(ctx context.Context, accountID, userID int64, p UpdateUserParams) (*User, error) {
	const q = `
		UPDATE users SET
			role = COALESCE($3, role),
			full_name = COALESCE($4, full_name),
			email = COALESCE($5, email),
			is_active = COALESCE($6, is_active)
		WHERE id = $1 AND account_id = $2
		RETURNING id, public_id, account_id, email, full_name, role, is_active, prefs, last_login_at, created_at`
	return scanUser(r.pool.QueryRow(ctx, q, userID, accountID, p.Role, p.FullName, p.Email, p.IsActive))
}

func (r *Repository) UpdatePrefs(ctx context.Context, userID int64, prefs []byte) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET prefs = $2 WHERE id = $1`, userID, prefs)
	return err
}

func (r *Repository) ClearUserLiveRefs(ctx context.Context, accountID, userID int64) error {
	if _, err := r.pool.Exec(ctx,
		`UPDATE leads SET assigned_user_id = NULL WHERE assigned_user_id = $1 AND owner_account_id = $2`,
		userID, accountID); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM lead_followers WHERE user_id = $1`, userID)
	return err
}

// Invites

func (r *Repository) CreateInvite(ctx context.Context, accountID int64, email, fullName, role, token string, expires time.Time) (*Invite, error) {
	const q = `INSERT INTO invites(account_id, email, full_name, role, token, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, account_id, email, full_name, role, expires_at, created_at`
	inv := &Invite{}
	err := r.pool.QueryRow(ctx, q, accountID, email, fullName, role, token, expires).Scan(
		&inv.ID, &inv.AccountID, &inv.Email, &inv.FullName, &inv.Role, &inv.ExpiresAt, &inv.CreatedAt)
	return inv, err
}

func (r *Repository) ListPendingInvites(ctx context.Context, accountID int64) ([]Invite, error) {
	const q = `SELECT id, account_id, email, full_name, role, expires_at, created_at
		FROM invites
		WHERE account_id = $1 AND accepted_at IS NULL AND expires_at > now()
		ORDER BY created_at`
	rows, err := r.pool.Query(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invite
	for rows.Next() {
		inv := Invite{}
		if err := rows.Scan(&inv.ID, &inv.AccountID, &inv.Email, &inv.FullName, &inv.Role, &inv.ExpiresAt, &inv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (r *Repository) FindPendingInviteByEmail(ctx context.Context, accountID int64, email string) (*Invite, error) {
	const q = `SELECT id, account_id, email, full_name, role, expires_at, created_at
		FROM invites
		WHERE account_id = $1 AND email = $2 AND accepted_at IS NULL AND expires_at > now()`
	inv := &Invite{}
	err := r.pool.QueryRow(ctx, q, accountID, email).Scan(
		&inv.ID, &inv.AccountID, &inv.Email, &inv.FullName, &inv.Role, &inv.ExpiresAt, &inv.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return inv, nil
}

func (r *Repository) GetPendingInvite(ctx context.Context, accountID, inviteID int64) (*InviteRow, error) {
	const q = `SELECT id, account_id, email, full_name, role, token, expires_at, false
		FROM invites
		WHERE id = $1 AND account_id = $2 AND accepted_at IS NULL AND expires_at > now()`
	row := &InviteRow{}
	err := r.pool.QueryRow(ctx, q, inviteID, accountID).Scan(
		&row.ID, &row.AccountID, &row.Email, &row.FullName, &row.Role, &row.Token, &row.ExpiresAt, &row.Accepted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return row, nil
}

func (r *Repository) UpdateInvite(ctx context.Context, accountID, inviteID int64, email, fullName, role, token *string, expires *time.Time) (*Invite, error) {
	const q = `
		UPDATE invites SET
			email = COALESCE($3, email),
			full_name = COALESCE($4, full_name),
			role = COALESCE($5, role),
			token = COALESCE($6, token),
			expires_at = COALESCE($7, expires_at)
		WHERE id = $1 AND account_id = $2 AND accepted_at IS NULL
		RETURNING id, account_id, email, full_name, role, expires_at, created_at`
	inv := &Invite{}
	err := r.pool.QueryRow(ctx, q, inviteID, accountID, email, fullName, role, token, expires).Scan(
		&inv.ID, &inv.AccountID, &inv.Email, &inv.FullName, &inv.Role, &inv.ExpiresAt, &inv.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return inv, nil
}

func (r *Repository) DeleteInvite(ctx context.Context, accountID, inviteID int64) error {
	ct, err := r.pool.Exec(ctx,
		`DELETE FROM invites WHERE id = $1 AND account_id = $2 AND accepted_at IS NULL`, inviteID, accountID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateBuyer inserts a buyer account, admin invite, balance row, and optional credit txn.
func (r *Repository) CreateBuyer(ctx context.Context, p CreateBuyerParams, token string, expires time.Time) (*CreateBuyerResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	a := &Account{}
	for range 10 {
		hid := handlerid.Generate("B")
		err = tx.QueryRow(ctx,
			`INSERT INTO accounts(type, name, website, timezone, handler_id) VALUES ('buyer', $1, $2, $3, $4)
			 RETURNING `+accountCols,
			p.Name, p.Website, p.Timezone, hid).Scan(
			&a.ID, &a.PublicID, &a.HandlerID, &a.Type, &a.Name, &a.Website, &a.Timezone, &a.CreatedAt)
		if err == nil {
			break
		}
		if !database.IsUniqueViolation(err) {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}

	adminName := strings.TrimSpace(p.AdminFirstName + " " + p.AdminLastName)
	if _, err := tx.Exec(ctx,
		`INSERT INTO invites(account_id, email, full_name, role, token, expires_at) VALUES ($1,$2,$3,'admin',$4,$5)`,
		a.ID, p.AdminEmail, adminName, token, expires); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO buyer_balances(buyer_id, balance) VALUES ($1, $2)`,
		a.ID, p.StartingBalance); err != nil {
		return nil, err
	}

	if p.StartingBalance > 0 {
		if _, err := tx.Exec(ctx,
			`INSERT INTO transactions(buyer_id, type, amount, balance_after, description)
			 VALUES ($1, 'credit', $2, $2, 'initial balance')`,
			a.ID, p.StartingBalance); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &CreateBuyerResult{
		Buyer: BuyerSummary{
			ID:        a.ID,
			PublicID:  a.PublicID,
			HandlerID: a.HandlerID,
			Name:      a.Name,
			Balance:   p.StartingBalance,
			LeadCount: 0,
		},
		InviteToken: token,
		AdminEmail:  p.AdminEmail,
	}, nil
}

type InviteRow struct {
	ID        int64
	AccountID int64
	Email     string
	FullName  string
	Role      string
	Token     string
	ExpiresAt time.Time
	Accepted  bool
}

func (r *Repository) FindInviteByToken(ctx context.Context, token string) (*InviteRow, error) {
	const q = `SELECT id, account_id, email, full_name, role, token, expires_at, (accepted_at IS NOT NULL)
		FROM invites WHERE token = $1`
	row := &InviteRow{}
	err := r.pool.QueryRow(ctx, q, token).Scan(
		&row.ID, &row.AccountID, &row.Email, &row.FullName, &row.Role, &row.Token, &row.ExpiresAt, &row.Accepted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return row, nil
}

// AcceptInvite creates the user and marks the invite accepted atomically.
func (r *Repository) AcceptInvite(ctx context.Context, inv *InviteRow, fullName, passwordHash string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var publicID string
	err = tx.QueryRow(ctx,
		`INSERT INTO users(account_id, email, password_hash, full_name, role)
		 VALUES ($1,$2,$3,$4,$5) RETURNING public_id`,
		inv.AccountID, inv.Email, passwordHash, fullName, inv.Role).Scan(&publicID)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE invites SET accepted_at = now() WHERE id = $1`, inv.ID); err != nil {
		return "", err
	}
	return publicID, tx.Commit(ctx)
}

// Password resets

func (r *Repository) CreatePasswordReset(ctx context.Context, userID int64, token string, expires time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO password_resets(user_id, token, expires_at) VALUES ($1,$2,$3)`,
		userID, token, expires)
	return err
}

type ResetRow struct {
	ID        int64
	UserID    int64
	ExpiresAt time.Time
	Used      bool
}

func (r *Repository) FindResetByToken(ctx context.Context, token string) (*ResetRow, error) {
	const q = `SELECT id, user_id, expires_at, (used_at IS NOT NULL) FROM password_resets WHERE token = $1`
	row := &ResetRow{}
	err := r.pool.QueryRow(ctx, q, token).Scan(&row.ID, &row.UserID, &row.ExpiresAt, &row.Used)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return row, nil
}

func (r *Repository) ConsumeReset(ctx context.Context, resetID, userID int64, passwordHash string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, passwordHash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE password_resets SET used_at = now() WHERE id = $1`, resetID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AdminContact is the primary admin user or pending invite for an account.
type AdminContact struct {
	FullName string
	Email    string
}

// PrimaryAdminContact returns the earliest admin user, or a pending admin invite if none accepted yet.
func (r *Repository) PrimaryAdminContact(ctx context.Context, accountID int64) (*AdminContact, error) {
	const userQ = `
		SELECT full_name, email FROM users
		WHERE account_id = $1 AND role = 'admin'
		ORDER BY created_at
		LIMIT 1`
	var c AdminContact
	err := r.pool.QueryRow(ctx, userQ, accountID).Scan(&c.FullName, &c.Email)
	if err == nil {
		return &c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	const invQ = `
		SELECT full_name, email FROM invites
		WHERE account_id = $1 AND role = 'admin' AND accepted_at IS NULL AND expires_at > now()
		ORDER BY created_at
		LIMIT 1`
	err = r.pool.QueryRow(ctx, invQ, accountID).Scan(&c.FullName, &c.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *Repository) ActiveAdminIDs(ctx context.Context, accountID int64) ([]int64, error) {
	return r.AdminUserIDs(ctx, r.pool, accountID)
}

// AdminUsersForAccount returns user IDs of active admins for notifications.
func (r *Repository) AdminUserIDs(ctx context.Context, q database.Querier, accountID int64) ([]int64, error) {
	rows, err := q.Query(ctx, `SELECT id FROM users WHERE account_id = $1 AND role = 'admin' AND is_active`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func scanUser(row pgx.Row) (*User, error) {
	u := &User{}
	err := row.Scan(&u.ID, &u.PublicID, &u.AccountID, &u.Email, &u.FullName,
		&u.Role, &u.IsActive, &u.Prefs, &u.LastLoginAt, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

package secure

import (
	"bytes"
	"errors"
	"time"
	
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const kidAccessV1 = "hs256:access:v1"
const kidRefreshV1 = "hs256:refresh:v1"

var ErrSecretsInvalid = errors.New("secrets must be at least 32 bytes")

// access Claims
type AccessClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

func NewAccessClaims(userId uuid.UUID, role string, claims jwt.RegisteredClaims) *AccessClaims {
	return &AccessClaims{UserID: userId, Role: role, RegisteredClaims: claims}
}

// refresh claims
type RefreshClaims struct {
	UserID uuid.UUID `json:"user_id"`
	jwt.RegisteredClaims
}

func NewRefreshClaims(userId uuid.UUID, claims jwt.RegisteredClaims) *RefreshClaims {
	return &RefreshClaims{UserID: userId, RegisteredClaims: claims}
}

// signer
type Signer struct {
	Issuer   string
	Audience string

	AccessSecret  []byte
	RefreshSecret []byte

	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

func NewSigner(iss, aud string, as, rs []byte, attl, rttl time.Duration) (*Signer, error) {
	if iss == "" || aud == "" {
		return nil, errors.New("issuer and aud must not be empty")
	}
	if len(as) < 32 || len(rs) < 32 {
		return nil, errors.New("secrets must be >= 32 bytes")
	}
	if bytes.Equal(as, rs) {
		return nil, errors.New("access and refresh secrets must be different")
	}
	if attl <= 0 || rttl <= 0 {
		return nil, errors.New("access and refresh TTL must be greater than 0")
	}
	return &Signer{iss, aud, as, rs, attl, rttl}, nil
}

// mint access
func (s *Signer) MintAccess(userId uuid.UUID, role string) (string, *AccessClaims, error) {
	now := time.Now().UTC()
	regClaims := jwt.RegisteredClaims{
		Issuer:   s.Issuer,
		Audience: []string{s.Audience},
		Subject:  userId.String(),

		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.AccessTTL)),

		ID: uuid.NewString(),
	}
	c := NewAccessClaims(userId, role, regClaims)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	tok.Header["kid"] = kidAccessV1
	tok.Header["typ"] = "JWT"
	signed, err := tok.SignedString(s.AccessSecret)
	return signed, c, err
}

// parse access
var (
	ErrUnknownKid   = errors.New("unknown kid")
	ErrInvalidToken = errors.New("invalid token")
)

func (s *Signer) ParseAccess(tok string) (*AccessClaims, error) {
	p := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuedAt(), jwt.WithExpirationRequired(), jwt.WithIssuer(s.Issuer),
		jwt.WithAudience(s.Audience), jwt.WithLeeway(30*time.Second),
	)
	var c AccessClaims
	token, err := p.ParseWithClaims(tok, &c, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		switch kid {
		case kidAccessV1:
			return s.AccessSecret, nil
		default:
			return nil, ErrUnknownKid
		}
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	return &c, nil
}

// mint refresh
func (s *Signer) MintRefresh(userId uuid.UUID) (string, *RefreshClaims, error) {
	now := time.Now().UTC()
	reqClaims := jwt.RegisteredClaims{
		Issuer:   s.Issuer,
		Audience: []string{s.Audience},
		Subject:  userId.String(),

		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.RefreshTTL)),

		ID: uuid.NewString(),
	}
	c := NewRefreshClaims(userId, reqClaims)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	tok.Header["kid"] = kidRefreshV1
	tok.Header["typ"] = "JWT"

	signed, err := tok.SignedString(s.RefreshSecret)
	return signed, c, err
}

// parse refresh
func (s *Signer) ParseRefresh(tok string) (*RefreshClaims, error) {
	p := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuedAt(), jwt.WithExpirationRequired(),
		jwt.WithIssuer(s.Issuer), jwt.WithAudience(s.Audience),
		jwt.WithLeeway(30*time.Second),
	)
	var c RefreshClaims
	token, err := p.ParseWithClaims(tok, &c, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		switch kid {
		case kidRefreshV1:
			return s.RefreshSecret, nil
		default:
			return nil, ErrUnknownKid
		}
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	return &c, nil
}

//

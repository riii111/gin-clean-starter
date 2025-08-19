//go:build unit

package user_test

import (
	"testing"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/tests/common/builder"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cmpOpts = []cmp.Option{
	cmpopts.IgnoreUnexported(user.User{}),
	cmpopts.EquateEmpty(),
}

type testCase struct {
	name   string
	mutate func(*builder.UserBuilder)
	errIs  error
}

func TestUser(t *testing.T) {
	t.Run("基本成功ケース", func(t *testing.T) {

		actual, err := builder.NewUserBuilder().BuildDomain()
		require.NoError(t, err)
		require.NotNil(t, actual)

		email, _ := user.NewEmail("test@example.com")
		role, _ := user.NewRole("admin")
		companyID := uuid.New()
		expected := user.NewUser(email, "hashed_password", role, &companyID)

		if diff := cmp.Diff(expected, actual, cmpOpts...); diff != "" {
			t.Errorf("User mismatch (-want +got):\n%s", diff)
		}

		assert.NotEqual(t, uuid.Nil, actual.ID())
		assert.True(t, actual.IsActive())
		assert.Nil(t, actual.LastLogin())
	})

	t.Run("メールアドレス検証", func(t *testing.T) {
		runCases(t, []testCase{
			{
				name:   "有効なメールアドレスOK",
				mutate: func(b *builder.UserBuilder) { b.WithEmail("valid@example.com") },
			},
			{
				name:   "空のメールアドレスNG",
				mutate: func(b *builder.UserBuilder) { b.WithEmail("") },
				errIs:  user.ErrInvalidEmail,
			},
			{
				name:   "無効な形式NG",
				mutate: func(b *builder.UserBuilder) { b.WithEmail("invalid-email") },
				errIs:  user.ErrInvalidEmail,
			},
			{
				name:   "@なしNG",
				mutate: func(b *builder.UserBuilder) { b.WithEmail("invalidemail.com") },
				errIs:  user.ErrInvalidEmail,
			},
		})
	})

	t.Run("ロール検証", func(t *testing.T) {
		runCases(t, []testCase{
			{
				name:   "admin ロールOK",
				mutate: func(b *builder.UserBuilder) { b.WithRole("admin") },
			},
			{
				name:   "operator ロールOK",
				mutate: func(b *builder.UserBuilder) { b.WithRole("operator") },
			},
			{
				name:   "viewer ロールOK",
				mutate: func(b *builder.UserBuilder) { b.WithRole("viewer") },
			},
			{
				name:   "無効なロールNG",
				mutate: func(b *builder.UserBuilder) { b.WithRole("invalid_role") },
				errIs:  user.ErrInvalidRole,
			},
			{
				name:   "空のロールNG",
				mutate: func(b *builder.UserBuilder) { b.WithRole("") },
				errIs:  user.ErrInvalidRole,
			},
		})
	})

	t.Run("会社ID検証", func(t *testing.T) {
		runCases(t, []testCase{
			{
				name: "会社ID有りOK",
				mutate: func(b *builder.UserBuilder) {
					companyID := uuid.New()
					b.WithCompanyID(&companyID)
				},
			},
			{
				name:   "会社ID無しOK",
				mutate: func(b *builder.UserBuilder) { b.WithoutCompany() },
			},
		})
	})

	t.Run("状態検証", func(t *testing.T) {
		runCases(t, []testCase{
			{
				name:   "アクティブユーザーOK",
				mutate: func(b *builder.UserBuilder) { /* デフォルトでアクティブ */ },
			},
			{
				name:   "非アクティブユーザーOK",
				mutate: func(b *builder.UserBuilder) { b.AsInactive() },
			},
		})
	})
}

func runCases(t *testing.T, cases []testCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {

			actual, err := builder.NewUserBuilder().With(c.mutate).BuildDomain()

			if c.errIs == nil {
				require.NotNil(t, actual)
				require.NoError(t, err)
			} else {
				require.Nil(t, actual)
				require.Error(t, err)
				require.ErrorIs(t, err, c.errIs)
			}
		})
	}
}

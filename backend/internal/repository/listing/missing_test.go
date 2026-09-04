package listingpg

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/listing"
)

func TestIsMissingRelation(t *testing.T) {
	t.Parallel()
	require.False(t, isMissingRelation(nil))
	require.False(t, isMissingRelation(errors.New("dial tcp timeout")))
	require.True(t, isMissingRelation(errors.New("ERROR: relation \"listings\" does not exist (SQLSTATE 42P01)")))
	require.True(t, isMissingRelation(errors.New("undefined_table")))
}

func TestPublicCatalogErr_MissingRelationIsEmpty(t *testing.T) {
	t.Parallel()
	items, total, err := publicCatalogErr(errors.New("ERROR: relation \"listings\" does not exist (SQLSTATE 42P01)"))
	require.NoError(t, err)
	require.Empty(t, items)
	require.Equal(t, 0, total)
}

func TestPublicCatalogErr_OtherErrorPropagates(t *testing.T) {
	t.Parallel()
	dial := errors.New("dial")
	items, total, err := publicCatalogErr(dial)
	require.ErrorIs(t, err, dial)
	require.Nil(t, items)
	require.Equal(t, 0, total)
}

func TestNormalisePagination(t *testing.T) {
	t.Parallel()
	page, size := normalisePagination(0, 0)
	require.Equal(t, 1, page)
	require.Equal(t, 20, size)
	page, size = normalisePagination(2, 200)
	require.Equal(t, 2, page)
	require.Equal(t, 20, size)
}

func TestToListing_DoesNotExposeOwnerOnPublicShape(t *testing.T) {
	t.Parallel()
	got := toListing(listingRow{
		ID: "id-1", OwnerAccountID: "owner-secret", State: string(listing.StatePublished),
		Title: "Furadeira", Description: "Furadeira de impacto 600W.",
		Category: "electric", PickupCity: "SP", OperatorName: "Carlos", OperatorPhone: "+5511",
	}, nil)
	require.Equal(t, "owner-secret", got.OwnerAccountID)
	require.Equal(t, "Carlos", got.Operator.Identity.Name)
}

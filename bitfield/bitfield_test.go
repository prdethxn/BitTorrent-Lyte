package bitfield

import (
	"testing"
)

func TestField(t *testing.T) {
	//Bitfield with 10 pieces
	bitfield := newBitfield(10)

	//Ensure nothing is intially marked as downlaoded
	for i := 0; i < 10; i++ {
		downloaded, err := isDownloaded(bitfield, i)
		if err != nil {
			t.Errorf("Unexpected error: for piece %d: %v", i, err)
		}

		if downloaded {
			t.Errorf("Piece %d shouldn't be marked as downloaded, intially", i)
		}
	}

	//Mark some pieces as downloaded
	markAsDownloaded(bitfield, 0)
	markAsDownloaded(bitfield, 3)
	markAsDownloaded(bitfield, 8)

	//Check downloaded peices
	for _, index := range []int{0, 3, 8} {
		downloaded, err := isDownloaded(bitfield, index)

		if err != nil {
			t.Errorf("Unexpected error for piece %d: %v", index, err)
		}

		if !downloaded {
			t.Errorf("Piece %d failed to download", index)
		}
	}

	//Check pieces that were not downloaded
	for _, index := range []int{1, 2, 4, 5, 6, 7, 9} {
		downloaded, err := isDownloaded(bitfield, index)

		if err != nil {
			t.Errorf("Unexpected error for piece %d: %v", index, err)
		}

		if downloaded {
			t.Errorf("Piece %d shouldn't be downloaded", index)
		}
	}

}

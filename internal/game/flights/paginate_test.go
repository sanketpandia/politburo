package flights

import "testing"

func TestPaginateReturnsRequestedWindow(t *testing.T) {
	items := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	page := Paginate(items, 2, 4)
	if len(page) != 4 || page[0] != 4 || page[3] != 7 {
		t.Fatalf("page = %#v", page)
	}
}

func TestPaginateClampsFinalPage(t *testing.T) {
	page := Paginate([]int{0, 1, 2}, 1, 50)
	if len(page) != 3 {
		t.Fatalf("page = %#v", page)
	}
}

func TestPaginatePastEndIsEmpty(t *testing.T) {
	page := Paginate([]int{0, 1}, 3, 50)
	if page == nil || len(page) != 0 {
		t.Fatalf("page = %#v, want empty slice", page)
	}
}

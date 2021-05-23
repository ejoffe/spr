package spr

import (
	"testing"
)

func TestSortPullRequestsSingleBranch(t *testing.T) {
	prs := []*pullRequest{
		{
			Number:     3,
			FromBranch: "third",
			ToBranch:   "second",
		},
		{
			Number:     2,
			FromBranch: "second",
			ToBranch:   "first",
		},
		{
			Number:     1,
			FromBranch: "first",
			ToBranch:   "master",
		},
	}

	sd := NewStackedPR(&Config{})
	prs = sd.sortPullRequests(prs)
	if prs[0].Number != 1 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[1].Number != 2 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[2].Number != 3 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
}

func TestSortPullRequestsTwoBranches(t *testing.T) {
	prs := []*pullRequest{
		{
			Number:     6,
			FromBranch: "b_third",
			ToBranch:   "b_second",
		},
		{
			Number:     5,
			FromBranch: "b_second",
			ToBranch:   "b_first",
		},
		{
			Number:     4,
			FromBranch: "b_first",
			ToBranch:   "master",
		},
		{
			Number:     3,
			FromBranch: "a_third",
			ToBranch:   "a_second",
		},
		{
			Number:     2,
			FromBranch: "a_second",
			ToBranch:   "a_first",
		},
		{
			Number:     1,
			FromBranch: "a_first",
			ToBranch:   "master",
		},
	}

	sd := NewStackedPR(&Config{})
	prs = sd.sortPullRequests(prs)
	if prs[0].Number != 4 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[1].Number != 5 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[2].Number != 6 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[3].Number != 1 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[4].Number != 2 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[5].Number != 3 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
}

func TestSortPullRequestsTwoBranchesMixed(t *testing.T) {
	prs := []*pullRequest{
		{
			Number:     6,
			FromBranch: "b_third",
			ToBranch:   "b_second",
		},
		{
			Number:     1,
			FromBranch: "a_first",
			ToBranch:   "master",
		},
		{
			Number:     5,
			FromBranch: "b_second",
			ToBranch:   "b_first",
		},
		{
			Number:     3,
			FromBranch: "a_third",
			ToBranch:   "a_second",
		},
		{
			Number:     4,
			FromBranch: "b_first",
			ToBranch:   "master",
		},
		{
			Number:     2,
			FromBranch: "a_second",
			ToBranch:   "a_first",
		},
	}

	sd := NewStackedPR(&Config{})
	prs = sd.sortPullRequests(prs)
	if prs[0].Number != 1 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[1].Number != 2 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[2].Number != 3 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[3].Number != 4 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[4].Number != 5 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[5].Number != 6 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
}

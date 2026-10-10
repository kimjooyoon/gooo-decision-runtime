package main

import "testing"

func TestCollisionCountsKeepDeclinesVisible(t *testing.T) {
	rows := []row{{Acceptable: 1, BeforeSHA: "same", AfterSHA: "one"}, {Acceptable: 2, BeforeSHA: "same", AfterSHA: "two"}, {Acceptable: 4, BeforeSHA: "same", Decline: "unknown"}}
	before, after, declined := countCollisions(rows)
	if before != 3 || after != 0 || declined != 2 {
		t.Fatal(before, after, declined)
	}
}

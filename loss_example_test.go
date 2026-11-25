package pagelayout_test

import (
	"fmt"

	"github.com/npillmayer/pagelayout"
)

func ExampleAssessPage() {
	score, err := pagelayout.AssessPage(pagelayout.DefaultLossPolicy(), pagelayout.PageAssessment{
		MainRemaining: true,
		Frames: []pagelayout.FrameAssessment{
			{Channel: "main", Width: 100, Height: 100, Occupied: 50},
			{Channel: "notes", Width: 100, Height: 100, Occupied: 0},
		},
		// Newly activated, incomplete target: carry now; underfill next attempt.
		PendingAfter: []pagelayout.PendingTarget{{ID: 1, Channel: "notes"}},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("underfill units:", score.Underfill.Info().Units)
	fmt.Println("carry units:", score.Carry.Info().Units)
	fmt.Println("total units:", score.Total.Info().Units)
	// Output:
	// underfill units: 250000
	// carry units: 1000000
	// total units: 2250000
}

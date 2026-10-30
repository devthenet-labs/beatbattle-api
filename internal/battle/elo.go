package battle

import "math"

// K is the Elo update factor used for every battle vote.
const K = 32.0

// Elo returns the new ratings for winner and loser after one battle vote
// between them, using the standard Elo formula with K=32 and the ratings
// as they stood before this vote. Both results are rounded to the
// nearest whole number.
func Elo(winner, loser int) (newWinner, newLoser int) {
	expectedWinner := 1 / (1 + math.Pow(10, float64(loser-winner)/400))
	newWinner = int(math.Round(float64(winner) + K*(1-expectedWinner)))
	newLoser = int(math.Round(float64(loser) - K*(1-expectedWinner)))
	return newWinner, newLoser
}

// Stakes returns the rating points side A and side B of a battle would
// each gain if they got the next vote, from ratingA and ratingB as they
// stand now, using the same Elo formula and K as Elo. An upset pays more:
// the lower-rated side's stake is the larger one.
func Stakes(ratingA, ratingB int) BattleStakes {
	newA, _ := Elo(ratingA, ratingB)
	newB, _ := Elo(ratingB, ratingA)
	return BattleStakes{A: newA - ratingA, B: newB - ratingB}
}

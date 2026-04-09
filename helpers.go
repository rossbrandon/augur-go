package augur

// Int returns a pointer to the given int value.
func Int(v int) *int { return &v }

// Float64 returns a pointer to the given float64 value.
func Float64(v float64) *float64 { return &v }

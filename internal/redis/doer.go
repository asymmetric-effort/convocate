package redis

// Doer is the interface required by RouterStore and DispatchStore. Both
// *Conn and *MockConn implement it.
type Doer interface {
	Do(args ...string) (interface{}, error)
	Close() error
}

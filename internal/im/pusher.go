package im

import "context"

// PushTarget is a concrete destination for an unsolicited message.
//
// Every field is filled in when a scheduled job is CREATED, from the
// conversation the user was in at the time — never assembled at delivery time
// from whatever the agent happens to say. That is deliberate: it means a job
// can only ever reach the chat it was born in, so "schedule a daily reminder
// for someone else" is not expressible, no matter what a prompt asks for.
type PushTarget struct {
	Platform Platform
	// ChatID is the group or conversation. Empty means a direct message to UserID.
	ChatID string
	// UserID is the recipient for direct messages, and the attribution for
	// group messages.
	UserID string
	// ThreadID keeps the message in a thread on platforms that have them.
	ThreadID string
}

// Pusher is an optional interface that adapters can implement to support
// sending messages that are not replies to an incoming message.
//
// The regular Adapter.SendReply path always answers an IncomingMessage, which
// carries the routing information the platform needs. A scheduled job has no
// incoming message — it fires on a timer — so platforms that support it need
// this separate entry point. Adapters that do not implement Pusher simply
// cannot be the destination of a scheduled job; the job is rejected at
// creation time rather than failing silently at 3am.
type Pusher interface {
	// Push sends an unsolicited message to the target.
	Push(ctx context.Context, target PushTarget, content string) error
}

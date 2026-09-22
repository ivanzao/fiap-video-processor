package events

const (
	AttributeEventType   = "eventType"
	AttributeTraceParent = "traceparent"
)

func MessageAttributes(env Envelope, traceparent string) map[string]string {
	attrs := map[string]string{AttributeEventType: env.EventType}
	if traceparent != "" {
		attrs[AttributeTraceParent] = traceparent
	}
	return attrs
}

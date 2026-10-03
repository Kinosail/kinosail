package identitycore

import "time"

// TimeoutsFor resolves browser validation limits from the persisted access channel.
func (sessions *Sessions) TimeoutsFor(session Session) (time.Duration, time.Duration) {
	if session.Channel == "public" && sessions.config.PublicTimeouts != nil {
		return sessions.config.PublicTimeouts()
	}
	return sessions.config.Timeouts()
}

func browserInactiveSeconds(browser bool, inactive time.Duration) int64 {
	if !browser {
		return 0
	}
	return int64(inactive / time.Second)
}

// LimitSession pins existing ceilings before a policy change. Increasing limits
// never extends authentication; tightening keeps sessions still within both limits.
func LimitSession(session Session, now int64, oldInactive, oldAbsolute, inactive, absolute time.Duration) (Session, bool) {
	if SessionExpired(session, now, oldInactive, oldAbsolute) {
		return session, false
	}
	if session.Browser {
		seconds := int64(min(oldInactive, inactive) / time.Second)
		if session.InactiveSeconds > 0 {
			seconds = min(seconds, session.InactiveSeconds)
		}
		session.InactiveSeconds = seconds
		session.ExpiresAt = min(session.ExpiresAt, session.CreatedAt+int64(min(oldAbsolute, absolute)/time.Second))
	} else if session.Channel == "public" {
		session.ExpiresAt = min(session.ExpiresAt, session.CreatedAt+int64(absolute/time.Second))
	}
	return session, !SessionExpired(session, now, inactive, absolute)
}

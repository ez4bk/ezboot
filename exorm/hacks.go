package exorm

//goland:noinspection GoVetUnsafePointer
func (s *Session) NoCheckVersion() *Session {
	s.db.NoVersionCheck()
	return s
}

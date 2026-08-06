package server

// registerRoutes is the single route-registration entry point. Each business
// domain owns its routes in a separate file to keep this method small.
func (s *HTTPServer) registerRoutes() {
	s.registerHealthRoutes()
	s.registerAdminRoutes()
	s.registerLLMRoutes()
}

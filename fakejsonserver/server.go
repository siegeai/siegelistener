package main

import "github.com/gorilla/mux"

type server struct {
	router  *mux.Router
	widgets []widget
}

func newServer() server {
	return server{
		router:  mux.NewRouter(),
		widgets: make([]widget, 0),
	}
}

func (s *server) populateTestWidgets() {
	s.widgets = []widget{
		{
			ID:          "1",
			Title:       "Jeremy Bearimy",
			Description: "Some convoluted cyclical timeline situation",
		},
		{
			ID:          "2",
			Title:       "Gizmo",
			Description: "This is more of a gizmo than a widget, but we'll abuse the widget system to store it here.",
		},
	}
}

type widget struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

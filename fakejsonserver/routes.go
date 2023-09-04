package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
	"io"
	"log"
	"net/http"
)

func (s *server) setupRoutes() {
	s.router.HandleFunc("/", s.handleRoot())
	s.router.HandleFunc("/widget", s.handleCreateWidget()).Methods("POST")
	s.router.HandleFunc("/widgets", s.handleGetWidgets()).Methods("GET")
	s.router.HandleFunc("/widgets/{id}", s.handleGetWidget()).Methods("GET")
	s.router.HandleFunc("/widgets/{id}", s.handleUpdateWidget()).Methods("PATCH")
	s.router.HandleFunc("/widgets/{id}", s.handleDeleteWidget()).Methods("DELETE")
	s.router.Use(logMiddleware)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := negroni.NewResponseWriter(w)
		next.ServeHTTP(ww, r)
		log.Println(r.Method, r.RequestURI, r.Proto, "->", ww.Status(), http.StatusText(ww.Status()))
	})
}

func (*server) handleRoot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Welcome home!")
	}
}

func (s *server) handleCreateWidget() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var newWidget widget
		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(w, "Kindly enter data with the widget title and description only in order to update")
		}

		json.Unmarshal(reqBody, &newWidget)
		s.widgets = append(s.widgets, newWidget)
		w.WriteHeader(http.StatusCreated)

		json.NewEncoder(w).Encode(newWidget)
	}
}

func (s *server) handleGetWidget() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		widgetID := mux.Vars(r)["id"]

		for _, singleWidget := range s.widgets {
			if singleWidget.ID == widgetID {
				json.NewEncoder(w).Encode(singleWidget)
			}
		}
	}
}

func (s *server) handleGetWidgets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(s.widgets)
	}
}

func (s *server) handleUpdateWidget() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		widgetId := mux.Vars(r)["id"]
		var updatedWidget widget

		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(w, "Kindly enter data with the widget title and description only in order to update")
		}

		json.Unmarshal(reqBody, &updatedWidget)

		for i, singleWidget := range s.widgets {
			if singleWidget.ID == widgetId {
				singleWidget.Title = updatedWidget.Title
				singleWidget.Description = updatedWidget.Description
				s.widgets = append(s.widgets[:i], singleWidget)
				json.NewEncoder(w).Encode(singleWidget)
			}
		}
	}
}

func (s *server) handleDeleteWidget() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		widgetId := mux.Vars(r)["id"]

		for i, singleWidget := range s.widgets {
			if singleWidget.ID == widgetId {
				s.widgets = append(s.widgets[:i], s.widgets[i+1:]...)
				fmt.Fprintf(w, "The widget with ID %v has been deleted successfully", widgetId)
			}
		}
	}
}

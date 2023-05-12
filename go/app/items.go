package main

type (
	Item struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		CategoryID    int    `json:"categoryID"`
		ImageFileName string `json:"imageFileName"`
	}
	Category struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
)

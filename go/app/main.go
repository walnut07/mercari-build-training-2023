package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	_ "github.com/mattn/go-sqlite3"
)

const (
	ImgDir         = "images"
	ImgDirRelative = "../" + ImgDir
	ItemFile       = "items.json"
	ItemsTable     = "../../db/items.db"
)

type (
	Response struct {
		Message string `json:"message"`
	}
	Items struct {
		Items []Item `json:"items"`
	}
	Item struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		Category      string `json:"category"`
		ImageFileName string `json:"imageFileName"`
	}
)

func root(c echo.Context) error {
	res := Response{Message: "Hello, world!"}
	return c.JSON(http.StatusOK, res)
}

func addItem(c echo.Context) error {
	// Get form data
	name := c.FormValue("name")
	category := c.FormValue("category")
	image, error := c.FormFile("image")
	if error != nil {
		return c.JSON(http.StatusBadRequest, error)
	}

	c.Logger().Infof("Receive item: %s", name)
	c.Logger().Infof("Receive category: %s", category)
	c.Logger().Infof("Receive image: %s", image.Filename)

	err := addItemToDatabase(name, category, image)
	if err != nil {
		c.Logger().Errorf("Failed to add item to database: %s", err)
		return c.JSON(http.StatusInternalServerError, err)
	}
	saveImageToLocal(image)

	message := fmt.Sprintf("item received: %s", name)
	res := Response{Message: message}

	return c.JSON(http.StatusOK, res)
}

func getItems(c echo.Context) error {
	jsonFile, err := os.Open(ItemFile)
	if err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	defer jsonFile.Close()

	jsonData, err := readItems()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	var items Items

	json.Unmarshal(jsonData, &items)

	return c.JSON(http.StatusOK, items)
}

func getItemByID(c echo.Context) error {
	id := c.Param("itemID")

	db, err := sql.Open("sqlite3", ItemsTable)
	if err != nil {
		c.Logger().Errorf("Failed to open database: %s", err)
		return err
	}

	row := db.QueryRow("SELECT * FROM items WHERE id = ?", id)
	var item Item
	err = row.Scan(&item.ID, &item.Name, &item.Category, &item.ImageFileName)
	if err != nil {
		if err.Error() == sql.ErrNoRows.Error() {
			c.Logger().Errorf("Item not found: %s", err)
			return c.JSON(http.StatusNotFound, err)
		} else {
			c.Logger().Errorf("Failed to get item: %s", err)
			return c.JSON(http.StatusInternalServerError, err)
		}
	}

	return c.JSON(http.StatusOK, item)
}

func getImg(c echo.Context) error {
	// Create image path
	imgPath := path.Join(ImgDir, c.Param("imageFilename"))

	if !strings.HasSuffix(imgPath, ".jpg") {
		res := Response{Message: "Image path does not end with .jpg"}
		return c.JSON(http.StatusBadRequest, res)
	}
	if _, err := os.Stat(imgPath); err != nil {
		c.Logger().Debugf("Image not found: %s", imgPath)
		imgPath = path.Join(ImgDir, "default.jpg")
	}

	return c.File(imgPath)
}

func searchItems(c echo.Context) error {
	keyword := c.QueryParam("keyword")

	db, err := sql.Open("sqlite3", ItemsTable)
	if err != nil {
		c.Logger().Errorf("Failed to open database: %s", err)
	}

	rows, err := db.Query("SELECT * FROM items WHERE name LIKE ? ", "%"+keyword+"%")
	if err != nil {
		c.Logger().Errorf("Failed to get items: %s", err)
		return c.JSON(http.StatusInternalServerError, err)
	}

	defer rows.Close()
	var items []Item
	for rows.Next() {
		var item Item
		err = rows.Scan(&item.ID, &item.Name, &item.Category, &item.ImageFileName)
		if err != nil {
			c.Logger().Errorf("Failed to get item: %s", err)
			return c.JSON(http.StatusInternalServerError, err)
		} else {
			items = append(items, item)
		}
	}

	return c.JSON(http.StatusOK, items)
}

func addItemToDatabase(name string, category string, image *multipart.FileHeader) error {
	hashedFileName := sha256.Sum256([]byte(image.Filename))
	ext := path.Ext(image.Filename)
	if ext != ".jpg" {
		return fmt.Errorf("image extension is not jpg")
	}

	db, err := sql.Open("sqlite3", ItemsTable)
	if err != nil {
		return err
	}

	statement, err := db.Prepare("CREATE TABLE IF NOT EXISTS items (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, category TEXT, imageFileName TEXT)")
	if err != nil {
		return err
	}
	statement.Exec()

	item := Item{}
	item.Name = name
	item.Category = category
	item.ImageFileName = fmt.Sprintf("%x.jpg", hashedFileName)

	statement, _ = db.Prepare("INSERT INTO items (name, category, imageFileName) VALUES (?, ?, ?)")
	statement.Exec(item.Name, item.Category, item.ImageFileName)

	return nil
}

func saveImageToLocal(image *multipart.FileHeader) {
	src, err := image.Open()
	if err != nil {
		fmt.Println("Cannot open image: ", err)
		return
	}
	defer src.Close()

	hashedName := sha256.Sum256([]byte(image.Filename))
	imgPath := path.Join(ImgDirRelative, fmt.Sprintf("%x.jpg", hashedName))

	dst, err := os.Create(imgPath)
	if err != nil {
		fmt.Println("Cannot create image: ", err)
		return
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		fmt.Println("Cannot copy image: ", err)
		return
	}
}

func readItems() ([]byte, error) {
	jsonFile, err := os.Open(ItemFile)
	if err != nil {
		return nil, err
	}

	defer jsonFile.Close()

	jsonData, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	return jsonData, nil
}

func main() {
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Logger.SetLevel(log.INFO)

	front_url := os.Getenv("FRONT_URL")
	if front_url == "" {
		front_url = "http://localhost:3000"
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{front_url},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	// Routes
	e.GET("/", root)
	e.GET("/items", getItems)
	e.GET("/items/:itemID", getItemByID)
	e.POST("/items", addItem)
	e.GET("/image/:imageFilename", getImg)
	e.GET("/search", searchItems)

	// Start server
	e.Logger.Fatal(e.Start(":9000"))
}

package main

import (
	"crypto/sha256"
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
)

const (
	ImgDir         = "images"
	ImgDirRelative = "../" + ImgDir
)

type (
	Response struct {
		Message string `json:"message"`
	}
	Items struct {
		Items []Item `json:"items"`
	}
	Item struct {
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

	updateJson(name, category, image)
	saveImageToLocal(image)

	message := fmt.Sprintf("item received: %s", name)
	res := Response{Message: message}

	return c.JSON(http.StatusOK, res)
}

func getItems(c echo.Context) error {
	jsonFile, err := os.Open("items.json")
	if err != nil {
		fmt.Println("Cannot open the json file", err)
		return err
	}
	defer jsonFile.Close()

	jsonData, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Println("Cannot read data", err)
		return err
	}

	var items Items

	json.Unmarshal(jsonData, &items)

	return c.JSON(http.StatusOK, items)
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

func updateJson(name string, category string, image *multipart.FileHeader) {
	hashedFileName := sha256.Sum256([]byte(image.Filename))

	jsonFile, err := os.Open("items.json")
	if err != nil {
		fmt.Println("Cannot open the json file: ", err)
		return
	}
	defer jsonFile.Close()

	jsonData, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Println("Cannot read data: ", err)
		return
	}

	var items Items

	json.Unmarshal(jsonData, &items)
	items.Items = append(items.Items, Item{Name: name, Category: category, ImageFileName: fmt.Sprintf("%x.jpg", hashedFileName)})
	marshaled, err := json.Marshal(items)
	if err != nil {
		fmt.Println("Cannot marshal data: ", err)
		return
	}
	if err = ioutil.WriteFile("items.json", marshaled, 0644); err != nil {
		fmt.Println("Cannot write data: ", err)
		return
	}
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
	e.POST("/items", addItem)
	e.GET("/image/:imageFilename", getImg)

	// Start server
	e.Logger.Fatal(e.Start(":9000"))
}

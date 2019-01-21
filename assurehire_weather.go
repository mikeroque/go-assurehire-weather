package main

import (
    "fmt"
	"io/ioutil"
    "log"
    "net/http"
	"html/template"
	"encoding/json"
	"bytes"
	"time"
	"strings"
	"github.com/patrickmn/go-cache"
)

// Make cache variable global
// so it can be accessed inside handleRequest
var c *cache.Cache

type TemplateContext struct {
	Debug string
	WeatherReport WeatherReport
	Errors []string
}

type WeatherReport struct {
	Data struct {
		Current_condition []struct {
			Temp_F string
			WeatherIconUrl []struct {
				Value string
			}
			WeatherDesc []struct {
				Value string
			}
		}
		Weather []Weather
		Error []struct {
			Msg string
		}
	}
	Valid bool
	Cached bool
}

type Weather struct {
	Date string
	MaxtempF string
	MintempF string
	Hourly []HourlyWeather
}

type HourlyWeather struct {
	WeatherIconUrl []ValueListField
	WeatherDesc []ValueListField
}

type ValueListField struct {
	Value string
}

// Declare interface to enable 
// override of default JSON marshalling
type Unmarshaler interface {
	UnmarshalJSON([]byte) error
}

// Define error struct
type LocationNotFoundError struct {
	Message string
}

// Add handler for error struct
func (e LocationNotFoundError) Error() string {
	return fmt.Sprintf("%v", e.Message)
}

// Override default JSON marshalling of Weather struct
func (weather *Weather) UnmarshalJSON(j []byte) error {
	var rawStrings map[string]interface{}

	// Unmarshal the JSON to a map of strings
	err := json.Unmarshal(j, &rawStrings)
	if err != nil {
		fmt.Printf("%v\n", err)
		return err
	}

	// Add each field to the struct
	for k, v := range rawStrings {
		if strings.ToLower(k) == "date" {
			// Format date as day of week
			timeDate, _ := time.Parse("2006-01-02", v.(string))
			weather.Date = timeDate.Weekday().String()[0:3]
		}else if strings.ToLower(k) == "mintempf" {
			weather.MintempF = v.(string)
		}else if strings.ToLower(k) == "maxtempf" {
			weather.MaxtempF = v.(string)
		
		// TODO: Add nodes that are an array
		// to get data for extended forecast
		// }else if strings.ToLower(k) == "hourly" {
			// weather.Hourly = v.([]HourlyWeather)
			// _ = append(hourly, arrayHourlyWeather[0])
		}
	}
	//weather.Hourly = hourly

	return nil
}

// Override default JSON marshalling of ValueListField struct
func (valueListField *ValueListField) UnmarshalJSON(j []byte) error {
	var rawStrings map[string]interface{}

	// Unmarshal the JSON to a map of strings
	err := json.Unmarshal(j, &rawStrings)
	if err != nil {
		fmt.Printf("%v\n", err)
		return err
	}

	// Add each field to the struct
	for k, v := range rawStrings {
		if strings.ToLower(k) == "weatherIconUrl" {
			valueListField.Value = v.(string)
		}else if strings.ToLower(k) == "weatherDesc" {
			valueListField.Value = v.(string)
		}
	}

	return nil
}

// Handle an HTTP request
func handleRequest(response http.ResponseWriter, req *http.Request) {	
	if req.URL.Path == "/" {
		var pageTemplateContext TemplateContext
		var weatherReport WeatherReport
		var weatherDataJSONString string
		
		switch req.Method {
			case "POST":
				// Populate req.Form
				err := req.ParseForm()
				if err != nil {
					panic(err)
				}
				
				// Get POST data
				f := req.Form
				//address := f.Get("address")
				//city := f.Get("city")
				//state := f.Get("state")
				zip := f.Get("zip")
				
				// Try to pull from cache
				weatherDataJSONInterface, found := c.Get(zip)
				
				if found {
					// Use cached data
					weatherDataJSONString = weatherDataJSONInterface.(string)
					
					// Set cached flag
					weatherReport.Cached = true
				}else{					
					// Fetch fresh data
					weatherDataJSONString, err = fetchWeatherData(zip)
				}
				
				// Unmarshal the JSON into a struct
				_ = json.Unmarshal([]byte(weatherDataJSONString), &weatherReport)
				
				// If an error occurred...
				if len(weatherReport.Data.Error) > 0 {
					// Add the message to the template context
					for _, weatherError := range weatherReport.Data.Error {
						pageTemplateContext.Errors = append(pageTemplateContext.Errors, weatherError.Msg)
					}
				}else{
					weatherReport.Valid = true
					
					// If not retrieved from cache...
					if !found {
						// Cache the JSON
						c.Set(zip, weatherDataJSONString, cache.DefaultExpiration)
					}
				}
				
				// Add JSON to template context
				pageTemplateContext.Debug = weatherDataJSONString
		}
		
		// Load the template
		pageTemplateFilename := "templates/index.html"
		pageTemplate, _ := template.ParseFiles(pageTemplateFilename)
		
		// Add weather data to the page context
		pageTemplateContext.WeatherReport = weatherReport
		
		// Parse the template using the context struct
		pageTemplate.Execute(response, pageTemplateContext)
	}
	
	return
}

func fetchWeatherData(query string) (string, error) {
	apiKey := "9803af8ac42a4c088fe31502192001"
	format := "JSON"
	apiUrl := "https://api.worldweatheronline.com/premium/v1/weather.ashx?q=" + query + "&key=" + apiKey + "&format=" + format
	json_string := ""
	
	// Fetch the data
	response, err := http.Get(apiUrl)
	if err != nil {
		log.Fatal(err)
	}
	
	// Unpack the response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}
	
	prettifyJSON := false
	if prettifyJSON {
		// Prettify JSON and cast to to string
		var json_string_buffer bytes.Buffer
		err = json.Indent(&json_string_buffer, body, "", "    ")
		json_string = json_string_buffer.String()
	}else{
		json_string = string(body)
	}
	
	return json_string, err
}

func main() {
	// Create the cache
	cacheDefaultExpiration := 30*time.Minute
	cachePurgeEvery := 1*time.Minute
	c = cache.New(cacheDefaultExpiration, cachePurgeEvery)

	fmt.Printf("Starting web server...\n")
	
	// Set root URL
    http.HandleFunc("/", handleRequest)
	
	// Launch web server
    log.Fatal(http.ListenAndServe(":8080", nil))
}

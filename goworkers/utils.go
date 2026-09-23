package goworkers

// utils funcs for every other funcs
import (
	"bytes"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"log"
	"os"
	"strings"
	"time"
)

// read any file
func readAnyFile(f string) string {
	fileContent, err := os.ReadFile(f)
	if err != nil {
		log.Fatalln("Error reading file " + f + ": " + err.Error())
	}
	return string(fileContent)
}

// read config json file
func readConfigFile(cf string) Config {
	var configContent Config
	err := json.Unmarshal([]byte(readAnyFile(cf)), &configContent)
	if err != nil {
		log.Fatalln("Error reading config file " + cf + ": " + err.Error())
	}
	return configContent
}

// hash string
func hashString(input string) string {
	hash := sha256.New()
	hash.Write([]byte(input))
	hashedBytes := hash.Sum(nil)
	return hex.EncodeToString(hashedBytes)
}

// parse html file
func parseHTMLTemplate(file string, keypool TemplateData) string {
	if keypool == nil {
		keypool = TemplateData{}
	}

	keypool["AppVersion"] = AppVersion

	tmpl, err := template.ParseFiles(file)
	if err != nil {
		log.Fatalln("Error parsing template failed: " + err.Error())

	}
	var renderedTemplate bytes.Buffer
	err = tmpl.Execute(&renderedTemplate, keypool)
	if err != nil {
		log.Fatalln("Error rendering template: " + err.Error())
	}
	return renderedTemplate.String()
}

// split srings by , and del spaces.
func splitCommaSeparated(input string) []string {
	parts := strings.Split(input, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// get time since unix clock start
func getUnixTimestamp() int64 {
	return time.Now().Unix()
}

// decode base64 string
func decodeB64String(encoded string) (decoded []byte) {
	decoded, err := b64.StdEncoding.DecodeString(encoded)
	if err != nil {
		log.Fatalln("Error de-base64ing string (" + encoded + "): " + err.Error())
	}
	return decoded
}

// encode byte array to b64
func encodeB64String(plain []byte) string {
	return b64.StdEncoding.EncodeToString(plain)
}

// translate str to lower
func strToLower(upperstr string) string {
	return strings.ToLower(upperstr)
}

// translate str to trim space
func strToTrimSpace(nottrimmed string) string {
	return strings.TrimSpace(nottrimmed)
}

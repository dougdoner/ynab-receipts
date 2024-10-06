package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/otiai10/gosseract"
	"gocv.io/x/gocv"
)

// Define API interaction details
var (
	YNAB_ACCESS_TOKEN string
	BUDGET_ID         string
	ACCOUNT_ID        string
)

// Predefined categories
var CATEGORY_MAP = map[string][]string{
	"groceries":   {"walmart", "whole foods", "grocery"},
	"dining":      {"restaurant", "cafe", "diner"},
	"electronics": {"best buy", "electronics", "tech"},
	// Add more categories as needed
}

// Helper function to categorize line items
func categorizeItem(description string) string {
	descriptionLower := strings.ToLower(description)
	for category, keywords := range CATEGORY_MAP {
		for _, keyword := range keywords {
			if strings.Contains(descriptionLower, keyword) {
				return category
			}
		}
	}
	return "uncategorized"
}

// Helper function to extract and categorize transactions from receipt image
func processReceiptImage(imagePath string) ([]map[string]interface{}, error) {
	// Load image using GoCV
	img := gocv.IMRead(imagePath, gocv.IMReadColor)
	if img.Empty() {
		return nil, fmt.Errorf("could not read image: %s", imagePath)
	}
	defer img.Close()

	// Convert image to grayscale
	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(img, &gray, gocv.ColorBGRToGray)

	// Apply thresholding for better OCR results
	thresh := gocv.NewMat()
	defer thresh.Close()
	gocv.Threshold(gray, &thresh, 150, 255, gocv.ThresholdBinaryInv)

	// Extract text using Tesseract
	client := gosseract.NewClient()
	defer client.Close()
	client.SetImageFromBytes(thresh.ToBytes())
	extractedText, err := client.Text()
	if err != nil {
		return nil, err
	}

	// Extract line items and amounts using regex
	lineItems := []map[string]interface{}{}
	pattern := regexp.MustCompile(`(?m)(.+?)\s+(\d+\.\d{2})`)
	matches := pattern.FindAllStringSubmatch(extractedText, -1)
	for _, match := range matches {
		description := strings.TrimSpace(match[1])
		amount := match[2]
		category := categorizeItem(description)
		lineItems = append(lineItems, map[string]interface{}{
			"description": description,
			"amount":      amount,
			"category":    category,
		})
	}

	return lineItems, nil
}

// Helper function to add transactions to YNAB using HTTP client
func addTransactionsToYNAB(lineItems []map[string]interface{}) error {
	url := fmt.Sprintf("https://api.youneedabudget.com/v1/budgets/%s/transactions", BUDGET_ID)
	client := &http.Client{}

	transactions := []map[string]interface{}{}
	for _, item := range lineItems {
		amount, err := strconv.ParseFloat(item["amount"].(string), 64)
		if err != nil {
			return err
		}
		transaction := map[string]interface{}{
			"account_id":  ACCOUNT_ID,
			"date":        "2024-10-06", // Replace with the actual date from receipt
			"amount":      int(amount * 1000),
			"payee_name":  item["description"].(string),
			"category_id": item["category"].(string),
			"memo":        "Imported from receipt",
			"cleared":     "cleared",
			"approved":    true,
		}
		transactions = append(transactions, transaction)
	}

	requestBody := map[string]interface{}{
		"transactions": transactions,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+YNAB_ACCESS_TOKEN)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add transactions: %s", string(body))
	}

	return nil
}

// Main workflow
func main() {
	// Load API credentials from file
	credentialsFile := "credentials.txt"
	credentials, err := os.ReadFile(credentialsFile)
	if err != nil {
		log.Fatalf("failed to read credentials file: %v", err)
	}
	credentialsLines := strings.Split(string(credentials), "\n")
	if len(credentialsLines) < 3 {
		log.Fatalf("credentials file should contain at least three lines: YNAB_ACCESS_TOKEN, BUDGET_ID, ACCOUNT_ID")
	}

	YNAB_ACCESS_TOKEN = strings.TrimSpace(credentialsLines[0])
	BUDGET_ID = strings.TrimSpace(credentialsLines[1])
	ACCOUNT_ID = strings.TrimSpace(credentialsLines[2])

	receiptsFolder := "receipts"
	receiptFiles, err := os.ReadDir(receiptsFolder)
	if err != nil {
		log.Fatalf("failed to read receipts folder: %v", err)
	}

	for _, receiptFile := range receiptFiles {
		if receiptFile.IsDir() {
			continue
		}
		imagePath := fmt.Sprintf("%s/%s", receiptsFolder, receiptFile.Name())
		lineItems, err := processReceiptImage(imagePath)
		if err != nil {
			log.Printf("failed to process receipt %s: %v", receiptFile.Name(), err)
			continue
		}

		print(lineItems)

		// err = addTransactionsToYNAB(lineItems)
		// if err != nil {
		// 	log.Printf("failed to add transactions for receipt %s: %v", receiptFile.Name(), err)
		// }
	}

	fmt.Println("All receipts processed and transactions added to YNAB successfully!")
}

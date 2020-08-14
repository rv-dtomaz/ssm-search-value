package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/cheggaaa/pb"
)

type ParameterDetail struct {
	Match        bool
	AwsParameter *ssm.ParameterMetadata
}

var defaultRegion string = "sa-east-1"

func main() {

	valueSearch := flag.String("value", "", "Value to search")
	region := flag.String("region", "", "Region")
	flag.Parse()

	if *region == "" {
		defaultRegion = *region
	}

	if *valueSearch == "" {
		fmt.Println("Please insert a value with --value")
		os.Exit(-1)
	}

	fmt.Println("Reading parameters list...")
	parameters, err := listParameters()
	if err != nil {
		fmt.Println("Error reading parameters: ", err)
		os.Exit(-1)
	}

	fmt.Println("Searching value...")
	err = filterParameters(parameters, *valueSearch)
	if err != nil {
		fmt.Println("Error reading parameters values: ", err)
		os.Exit(-1)
	}

}

func filterParameters(input []*ssm.ParameterMetadata, valueFilter string) error {

	ssmService := getSSMService()

	resultErrors := []string{}
	resultFound := []string{}
	processedPaths := map[string]bool{}

	total := len(input)
	current := 0

	// create and start new bar
	bar := pb.StartNew(total)
	bar.AlwaysUpdate = true
	for _, item := range input {

		//	fmt.Printf("Reading %d of %d\n", current, total)
		bar.Increment()
		current++
		if _, ok := processedPaths[*item.Name]; ok {
			continue
		}

		if strings.HasPrefix(*item.Name, "/") {

			startPath := strings.Split(*item.Name, "/")[1]
			getInputByPath := &ssm.GetParametersByPathInput{
				Path:           aws.String("/" + startPath),
				Recursive:      aws.Bool(true),
				WithDecryption: aws.Bool(true),
			}

			err := ssmService.GetParametersByPathPages(getInputByPath,
				func(page *ssm.GetParametersByPathOutput, lastPage bool) bool {

					for _, itemP := range page.Parameters {
						if *itemP.Value == valueFilter {
							resultFound = append(resultFound, *item.Name)

						}
						processedPaths[*itemP.Name] = true
					}

					return true
				})
			if err != nil {
				resultErrors = append(resultErrors, fmt.Sprintf("Error getting parameter for path [%s]--->%v\n", *item.Name, err))
				continue
			}

		} else {

			time.Sleep(100)

			getInputParameter := ssm.GetParameterInput{
				Name:           item.Name,
				WithDecryption: aws.Bool(true),
			}

			param, err := ssmService.GetParameter(&getInputParameter)
			if err != nil {
				resultErrors = append(resultErrors, fmt.Sprintf("Error getting parameter [%s]--->%v\n", *item.Name, err))
				continue
			}

			if *param.Parameter.Value == valueFilter {
				resultFound = append(resultFound, *item.Name)
			}

			processedPaths[*item.Name] = true
		}

	}

	fmt.Println("------------- ERRORS -------------------")
	for _, item := range resultErrors {
		fmt.Println(item)
	}
	fmt.Println("------------- ERRORS -------------------")
	fmt.Println()

	fmt.Println("------------- KEYS USING VALUE -------------------")
	for _, item := range resultFound {
		fmt.Println(item)
	}
	fmt.Println("------------- KEYS USING VALUE -------------------")
	fmt.Println()

	return nil
}

func getSSMService() *ssm.SSM {

	cfg := aws.NewConfig().WithRegion(defaultRegion)
	cfg.DisableRestProtocolURICleaning = aws.Bool(true)
	ssmService := ssm.New(session.New(), cfg)

	return ssmService
}

func listParameters() ([]*ssm.ParameterMetadata, error) {

	ssmService := getSSMService()

	filter := make([]*ssm.ParametersFilter, 0)
	filter = append(filter, &ssm.ParametersFilter{
		Key:    aws.String("Type"),
		Values: aws.StringSlice([]string{"String", "SecureString"}),
	})

	describeParametersInput := &ssm.DescribeParametersInput{
		Filters:    filter,
		MaxResults: aws.Int64(50),
	}

	result := make([]*ssm.ParameterMetadata, 0)

	for {

		res, err := ssmService.DescribeParameters(describeParametersInput)
		if err != nil {
			return nil, err
		}

		result = append(result, res.Parameters...)
		if res.NextToken == nil {
			break
		} else {
			describeParametersInput.NextToken = res.NextToken
		}

	}

	return result, nil

}

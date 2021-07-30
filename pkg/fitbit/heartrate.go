package fitbit

// Sample:
//
// {
//     "activities-heart": [
//         {
//             "customHeartRateZones": [],
//             "dateTime": "today",
//             "heartRateZones": [
//                 {
//                     "caloriesOut": 0,
//                     "max": 220,
//                     "min": 167,
//                     "minutes": 0,
//                     "name": "Peak"
//                 }
//             ],
//             "value": "99.58"
//         }
//     ],
//     "activities-heart-intraday": {
//         "dataset": [
//             {
//                 "time": "13:08:41",
//                 "value": 122
//             }
//         ],
//         "datasetInterval": 1,
//         "datasetType": "second"
//     }
// }
type HeartRateResult struct {
	Activities         []HeartActivity       `json:"activities-heart"`
	ActivitiesIntraDay HeartActivityIntraday `json:"activities-heart-intraday"`
}

// Sample:
//
// {
// 	"customHeartRateZones": [],
// 	"dateTime": "today",
// 	"heartRateZones": [
// 		{
// 			"caloriesOut": 55.26554,
// 			"max": 118,
// 			"min": 30,
// 			"minutes": 21,
// 			"name": "Out of Range"
// 		}
// 	],
// 	"value": "99.58"
// }
type HeartActivity struct {
	CustomHeartRateZones []HeartRateZone `json:"customHeartRateZones"`
	DateTime             string          `json:"dateTime"`
	HeartRateZones       []HeartRateZone `json:"heartRateZones"`
	Value                string          `json:"value"`
}

// Sample:
//
// {
// 	"dataset": [
// 		{
// 			"time": "13:00:01",
// 			"value": 91
// 		}
// 	],
// 	"datasetInterval": 1,
// 	"datasetType": "second"
// }
type HeartActivityIntraday struct {
	Dataset         []HeartActivityIntradayDatasetValue `json:"dataset"`
	DatasetInterval int                                 `json:"datasetInterval"`
	DatasetType     string                              `json:"datasetType"`
}

// Sample:
//
// {
// 		"time": "13:00:01",
// 		"value": 91
// }
type HeartActivityIntradayDatasetValue struct {
	Time  string `json:"time"`
	Value int    `json:"value"`
}

// Sample:
//
// {
// 	"caloriesOut": 55.26554,
// 	"max": 118,
// 	"min": 30,
// 	"minutes": 21,
// 	"name": "Out of Range"
// }
type HeartRateZone struct {
	CaloriesOut float64 `json:"caloriesOut"`
	Max         int     `json:"max"`
	Min         int     `json:"min"`
	Minutes     int     `json:"minutes"`
	Name        string  `json:"name"`
}

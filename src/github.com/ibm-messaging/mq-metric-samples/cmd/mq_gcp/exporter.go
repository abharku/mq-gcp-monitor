package main

/*
  Copyright (c) IBM Corporation 2016

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific

   Contributors:
     Mark Taylor - Initial Contribution
*/

/*
The Collect() function is the key operation
invoked at the configured intervals, causing us to read available publications
and update the various data points.
*/

import (
	"fmt"
	"unicode"
	"context"
	ibmmq "github.com/ibm-messaging/mq-golang/v5/ibmmq"

	"github.com/ibm-messaging/mq-golang/v5/mqmetric"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
	monitoring "cloud.google.com/go/monitoring/apiv3"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredres "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

var (
	first              = true
	errorCount         = 0
	lastPoll           = time.Now()
	lastQueueDiscovery time.Time
	platformString     = ""
)
const metricType = "custom.googleapis.com/custom_measurement"
/*
Collect is called by the main routine at regular intervals to provide current
data
*/
func Collect() error {
	var err error
	series := ""
	log.Infof("IBM MQ stdout collector started")
	if platformString == "" {
		platformString = strings.Replace(ibmmq.MQItoString("PL", int(mqmetric.GetPlatform())), "MQPL_", "", -1)
	}

	collectStartTime := time.Now()

	// Do we need to poll for object status on this iteration
	pollStatus := false
	thisPoll := time.Now()
	elapsed := thisPoll.Sub(lastPoll)
	if elapsed >= config.cf.PollIntervalDuration || first {
		log.Debugf("Polling for object status")
		lastPoll = thisPoll
		pollStatus = true
	}

	// Clear out everything we know so far. In particular, replace
	// the map of values for each object so the collection starts
	// clean.
	for _, cl := range mqmetric.Metrics.Classes {
		for _, ty := range cl.Types {
			for _, elem := range ty.Elements {
				elem.Values = make(map[string]int64)
			}
		}
	}

	// Process all the publications that have arrived
	err = mqmetric.ProcessPublications()
	if err != nil {
		log.Fatalf("Error processing publications: %v", err)
	}
	// If there has been sufficient interval since the last explicit poll for
	// status, then do that collection too
	if pollStatus {
		if config.cf.CC.UseStatus {
			err := mqmetric.CollectQueueManagerStatus()
			if err != nil {
				log.Errorf("Error collecting queue manager status: %v", err)
			} else {
				log.Debugf("Collected all queue manager status")
			}
			err = mqmetric.CollectChannelStatus(config.cf.MonitoredChannels)
			if err != nil {
				log.Errorf("Error collecting channel status: %v", err)
			} else {
				log.Debugf("Collected all channel status")
			}
			err = mqmetric.CollectTopicStatus(config.cf.MonitoredTopics)
			if err != nil {
				log.Errorf("Error collecting topic status: %v", err)
			} else {
				log.Debugf("Collected all topic status")
			}
			err = mqmetric.CollectSubStatus(config.cf.MonitoredSubscriptions)
			if err != nil {
				log.Errorf("Error collecting subscription status: %v", err)
			} else {
				log.Debugf("Collected all subscription status")
			}

			err = mqmetric.CollectQueueStatus(config.cf.MonitoredQueues)
			if err != nil {
				log.Errorf("Error collecting queue status: %v", err)
			} else {
				log.Debugf("Collected all queue status")
			}

			if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
				err = mqmetric.CollectUsageStatus()
				if err != nil {
					log.Errorf("Error collecting bufferpool/pageset status: %v", err)
				} else {
					log.Debugf("Collected all buffer pool/pageset status")
				}
			}
		}
	}

	thisDiscovery := time.Now()
	elapsed = thisDiscovery.Sub(lastQueueDiscovery)
	if config.cf.RediscoverDuration > 0 {
		if elapsed >= config.cf.RediscoverDuration {
			err = mqmetric.RediscoverAndSubscribe(discoverConfig)
			lastQueueDiscovery = thisDiscovery
			err = mqmetric.RediscoverAttributes(ibmmq.MQOT_CHANNEL, config.cf.MonitoredChannels)
		}
	}
	// Have now processed all of the publications, and all the MQ-owned
	// value fields and maps have been updated.
	//
	// Now need to set all of the real items with the correct values
	if first {
		// Always ignore the first loop through as there might
		// be accumulated stuff from a while ago, and lead to
		// a misleading range on graphs.
		first = false
	} else {
		// Start with a metric that shows how many publications were processed by this collection
		series = "qmgr"
		tags := map[string]string{
			"qmgr":     config.cf.QMgrName,
			"platform": platformString,
		}
		printPoint(series, "exporter_publications", float32(mqmetric.GetProcessPublicationCount()), tags)

		for _, cl := range mqmetric.Metrics.Classes {
			for _, ty := range cl.Types {
				for _, elem := range ty.Elements {
					for key, value := range elem.Values {
						f := mqmetric.Normalise(elem, key, value)
						tags = map[string]string{
							"qmgr": config.cf.QMgrName,
						}

						series = "qmgr"
						if key != mqmetric.QMgrMapKey {
							series = "queue"
							tags[series] = key
						}
						printPoint(series, elem.MetricName, float32(f), tags)
					}
				}
			}
		}
	}
	series = "channel"
	for _, attr := range mqmetric.ChannelStatus.Attributes {
		for key, value := range attr.Values {
			if value.IsInt64 {

				chlType := int(mqmetric.ChannelStatus.Attributes[mqmetric.ATTR_CHL_TYPE].Values[key].ValueInt64)
				chlTypeString := strings.Replace(ibmmq.MQItoString("CHT", chlType), "MQCHT_", "", -1)
				// Not every channel status report has the RQMNAME attribute (eg SVRCONNs)
				rqmName := ""
				if rqmNameAttr, ok := mqmetric.ChannelStatus.Attributes[mqmetric.ATTR_CHL_RQMNAME].Values[key]; ok {
					rqmName = rqmNameAttr.ValueString
				}

				chlName := mqmetric.ChannelStatus.Attributes[mqmetric.ATTR_CHL_NAME].Values[key].ValueString
				connName := mqmetric.ChannelStatus.Attributes[mqmetric.ATTR_CHL_CONNNAME].Values[key].ValueString
				jobName := mqmetric.ChannelStatus.Attributes[mqmetric.ATTR_CHL_JOBNAME].Values[key].ValueString

				tags := map[string]string{
					"qmgr": config.cf.QMgrName,
				}
				tags["channel"] = chlName
				tags["platform"] = platformString
				tags[mqmetric.ATTR_CHL_TYPE] = strings.TrimSpace(chlTypeString)
				tags[mqmetric.ATTR_CHL_RQMNAME] = strings.TrimSpace(rqmName)
				tags[mqmetric.ATTR_CHL_CONNNAME] = strings.TrimSpace(connName)
				tags[mqmetric.ATTR_CHL_JOBNAME] = strings.TrimSpace(jobName)

				f := mqmetric.ChannelNormalise(attr, value.ValueInt64)
				printPoint(series, attr.MetricName, float32(f), tags)
			}
		}
	}

	series = "queue"
	for _, attr := range mqmetric.QueueStatus.Attributes {
		for key, value := range attr.Values {
			if value.IsInt64 {

				qName := mqmetric.QueueStatus.Attributes[mqmetric.ATTR_Q_NAME].Values[key].ValueString

				tags := map[string]string{
					"qmgr": config.cf.QMgrName,
				}
				tags["queue"] = qName
				tags["platform"] = platformString
				usage := ""
				if usageAttr, ok := mqmetric.QueueStatus.Attributes[mqmetric.ATTR_Q_USAGE].Values[key]; ok {
					if usageAttr.ValueInt64 == 1 {
						usage = "XMITQ"
					} else {
						usage = "NORMAL"
					}
				}

				tags["usage"] = usage
				tags["description"] = mqmetric.GetObjectDescription(key, ibmmq.MQOT_Q)

				f := mqmetric.QueueNormalise(attr, value.ValueInt64)
				printPoint(series, attr.MetricName, float32(f), tags)
			}
		}
	}

	series = "topic"
	for _, attr := range mqmetric.TopicStatus.Attributes {
		for key, value := range attr.Values {
			if value.IsInt64 {

				topicString := mqmetric.TopicStatus.Attributes[mqmetric.ATTR_TOPIC_STRING].Values[key].ValueString
				topicStatusType := mqmetric.TopicStatus.Attributes[mqmetric.ATTR_TOPIC_STATUS_TYPE].Values[key].ValueString

				tags := map[string]string{
					"qmgr": config.cf.QMgrName,
				}
				tags["topic"] = topicString
				tags["platform"] = platformString
				tags["type"] = topicStatusType

				f := mqmetric.TopicNormalise(attr, value.ValueInt64)
				printPoint(series, attr.MetricName, float32(f), tags)
			}
		}
	}

	series = "subscription"
	for _, attr := range mqmetric.SubStatus.Attributes {
		for key, value := range attr.Values {
			if value.IsInt64 {
				subId := mqmetric.SubStatus.Attributes[mqmetric.ATTR_SUB_ID].Values[key].ValueString
				subName := mqmetric.SubStatus.Attributes[mqmetric.ATTR_SUB_NAME].Values[key].ValueString
				subType := int(mqmetric.SubStatus.Attributes[mqmetric.ATTR_SUB_TYPE].Values[key].ValueInt64)
				subTypeString := strings.Replace(ibmmq.MQItoString("SUBTYPE", subType), "MQSUBTYPE_", "", -1)
				topicString := mqmetric.SubStatus.Attributes[mqmetric.ATTR_SUB_TOPIC_STRING].Values[key].ValueString

				tags := map[string]string{
					"qmgr": config.cf.QMgrName,
				}

				tags["platform"] = platformString
				tags["type"] = subTypeString
				tags["subid"] = subId
				tags["subscription"] = subName
				tags["topic"] = topicString
				f := mqmetric.SubNormalise(attr, value.ValueInt64)
				printPoint(series, attr.MetricName, float32(f), tags)
			}
		}
	}

	series = "qmgr"
	for _, attr := range mqmetric.QueueManagerStatus.Attributes {
		for _, value := range attr.Values {
			if value.IsInt64 {

				qMgrName := strings.TrimSpace(config.cf.QMgrName)

				tags := map[string]string{
					"qmgr":     qMgrName,
					"platform": platformString,
				}

				f := mqmetric.QueueManagerNormalise(attr, value.ValueInt64)
				printPoint(series, attr.MetricName, float32(f), tags)
			}
		}
	}

	if mqmetric.GetPlatform() == ibmmq.MQPL_ZOS {
		series = "bufferpool"
		for _, attr := range mqmetric.UsageBpStatus.Attributes {
			for key, value := range attr.Values {
				bpId := mqmetric.UsageBpStatus.Attributes[mqmetric.ATTR_BP_ID].Values[key].ValueString
				bpLocation := mqmetric.UsageBpStatus.Attributes[mqmetric.ATTR_BP_LOCATION].Values[key].ValueString
				bpClass := mqmetric.UsageBpStatus.Attributes[mqmetric.ATTR_BP_CLASS].Values[key].ValueString
				if value.IsInt64 && !attr.Pseudo {
					qMgrName := strings.TrimSpace(config.cf.QMgrName)

					tags := map[string]string{
						"qmgr":     qMgrName,
						"platform": platformString,
					}
					tags["bufferpool"] = bpId
					tags["location"] = bpLocation
					tags["pageclass"] = bpClass

					f := mqmetric.UsageNormalise(attr, value.ValueInt64)
					printPoint(series, attr.MetricName, float32(f), tags)
				}
			}
		}

		series = "pageset"
		for _, attr := range mqmetric.UsagePsStatus.Attributes {
			for key, value := range attr.Values {
				qMgrName := strings.TrimSpace(config.cf.QMgrName)
				psId := mqmetric.UsagePsStatus.Attributes[mqmetric.ATTR_PS_ID].Values[key].ValueString
				bpId := mqmetric.UsagePsStatus.Attributes[mqmetric.ATTR_PS_BPID].Values[key].ValueString
				if value.IsInt64 && !attr.Pseudo {
					tags := map[string]string{
						"qmgr":     qMgrName,
						"platform": platformString,
					}
					tags["pageset"] = psId
					tags["bufferpool"] = bpId
					f := mqmetric.UsageNormalise(attr, value.ValueInt64)
					printPoint(series, attr.MetricName, float32(f), tags)
				}
			}
		}
	}

	collectStopTime := time.Now()
	elapsedSecs := int64(collectStopTime.Sub(collectStartTime).Seconds())
	log.Infof("Collection time = %d secs", elapsedSecs)

	return err

}

// Athough the tags are the same map contents as other exporters in this repo,
// only the qmgr name and the object name are actually used. So we lose all the
// other tag data describing the point.
func printPoint(series string, metric string, val float32, tags map[string]string) {
    log.Infof("Trying to print point")
	//time.Sleep(1 * time.Second)
        var y int64 = int64(val)
	metric = series + "_" + metric

	// For subscriptions the useful identifier is the topic name
	if series == "subscription" {
		series = "topic"
	}
	qmgr := tags["qmgr"]
	if obj, ok := tags[series]; ok {
		metric += "-" + sanitiseString(obj)
		if obj == "" {
			fmt.Printf("Object %s empty value %+v\n", metric, tags)
		}
	}
	ctx := context.Background()
	c, err := monitoring.NewMetricClient(ctx)
	if err != nil {
	log.Errorf("Error collecting queue status: %v", err)
		return 
	}
	nows := &timestamp.Timestamp{
		Seconds: time.Now().Unix(),
	}
	req := &monitoringpb.CreateTimeSeriesRequest{
		Name: "projects/" + config.projectId,
		TimeSeries: []*monitoringpb.TimeSeries{{
			Metric: &metricpb.Metric{
				Type: metricType,
				Labels: map[string]string{
					"qmgr": sanitiseString(qmgr),
                                        "metric": metric,
				},
			},
			Resource: &monitoredres.MonitoredResource{
				Type: "gce_instance",
				Labels: map[string]string{
					"instance_id": config.hostlabel,
					"zone":        config.zone,
				},
			},
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					StartTime: nows,
					EndTime:   nows,
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{
						Int64Value: y,
					},
				},
			}},
		}},
	}
	err = c.CreateTimeSeries(ctx, req)
	if err != nil {
	log.Errorf("Error collecting queue status: %v", err)
		return 
	}
        err = c.Close()
	fmt.Printf("PUTVAL %s/%s-%s/%s interval=%s N:%f\n",
		config.hostlabel, "qmgr", sanitiseString(qmgr), metric, config.interval, val)
	return
}

func fixup(s1 string) string {
	s2 := strings.Replace(s1, ".", "_", -1)
	return s2
}

// Only the following characters are allowed in names: a to z, A to Z, 0 to 9, -, _, .,
func sanitiseString(s string) string {
	r := make([]rune, len(s))
	i := 0
	for _, c := range s {
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' {
			r[i] = c
		} else if c == '.' || c == '-' {
			r[i] = '_'
		} else {
			r[i] = '_'
		}
		i++
	}

	// Make sure tag is not empty
	s2 := string(r[:i])
	if s2 == "" {
		s2 = "-"
	}
	return s2
}

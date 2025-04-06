# catchall-assessment

## Introduction 
Everyday at Mailgun we build and maintain systems at scale. For example our email delivery system generates various types of events that describe the various statuses of an email as it passes through our system. At last count we produce around 200k of these events every second, with this number growing by about 60% every year. Many of the services we build must be designed to handle the current level of traffic and be built to scale into the future. To help give you a taste for this scale and to understand your skills in this area, We would like to invite you to design and code a highly scalable event processing system.

## Catch-all Domain
A Catch-all domain is a domain name that ensures no email to the domain is rejected or lost. With a catch-all domain, you could tell people to send email to anything at your designated domain such as: devs@example.com, info@example.com, foo@example, or younameit@example.com. No matter what they entered in front of the @ sign, email sent to this address will never bounce or get rejected. Though useful for those concerned about potentially missing important messages due to typos in the mailbox, spammers take advantage of them, as they do not need to hunt for usernames, guess mailbox names, or scrape email addresses. They simply put whatever they want in front of the domain and send their messages — and those messages arrive as intended. As a result, catch-all domains tend to get flooded with spam and become unusable.

## Challenge 
Your code challenge, should you choose to accept it, is to build a distributed system that identifies these catch-all domains by counting delivered and bounced events (https://en.wikipedia.org/wiki/Bounce_message) provided to your application through the following endpoints:

- PUT /events/<domain_name>/delivered - receives an event of type “delivered”. 
- PUT /events/<domain_name>/bounced - receives an event of type “bounced”.

For this challenge the testing events can be generated using the provided code in this repository.

A domain name can be considered a catch-all when it receives more than 1000 “delivered” events. If a domain name receives less than 1000 “delivered” events it’s considered “unknown” as there’s not enough data to make a determination. A domain name is not a catch-all when it receives at least 1 “bounced” event. The customers should be able to use the following API to query the status of a domain name:

- GET /domains/<domain_name> - returns whether a domain name is catch-all / not catch-all / unknown.

The service should have a fault tolerant architecture that is able to scale to handle 100,000 events per second across 10,000,000 different domain names with P95 latency of 200ms. It’s important to note that your solution doesn’t have to work at that scale, it merely has to have the architecture capable of doing so. We will accept architectures capable of smaller scale with supporting documentation outlining the thoughts on how to improve it.

We're looking for the service to be written in Go. Feel free to bring libraries and frameworks, ideally lightweight ones. No need for over-engineering, we just need to see you get the basics right, we can iterate as we go along.

## App Requirements 
- Being able to be deployed on multiple nodes and scaled horizontally.
- One of the following databases should be used: MongoDB, Cassandra, Clickhouse.
- Code Requirements Close-to-production quality code, whatever this means to you (unit-tests), however no need to create a deployment process.
- During the code review you are free to make code changes in response to feedback.

## Bonus Points 
Anything you think would make this application better.

## Timeline 
Please have this excise done within a week of recieving it. If you need more time or have questions please don't hesitate to reach out.

Thanks!

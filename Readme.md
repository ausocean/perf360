# Readme

Perf360 is a simple web app for collecting 360-degree performance
reviews. Written in Go, Perf360 is designed to run on Google App
Engine but can also run locally.

## Usage

Any user can write a peer review about any other user. Once all
reviews have been submitted, reviews become available to the intended
recipient, their manager, and the CEO.

## Environment Variables

Perf360 is configured via five environment variables:

* PERF360_SECRETS contains the OAuth2 client secret (clientSecret) and the OAuth2 cookie session key (sessionKey).
* PREF360_USERS contains in users in JSON format. See below.
* PERF360_CREDENTIALS contains the datastore client credentials. Only required when running locally.
* PERF360_PERIOD is the review period (typically a year).
* PERF360_STATUS is either **open** (when editing peer reviews) or **closed** (when reviewing peer reviews).

## Process

* Start the review process by setting PERF360_PERIOD to the review period, i.e., the year.
* Set PERF360_STATUS to **open**.
* Once all reviews have been submitted, set PERF360_STATUS to **closed**.
* Peer reviews are then available for viewing.

## Users file format

The JSON file format for importing users is as follows:

```
[
    {"Name": "Jane", "Email": "jane@acme.org", "IsManager": true, "IsCEO": true},
    {"Name": "Hiro", "Email": "hiro@acme.org", "IsManager": true, "Manager": "Jane"},
    {"Name": "Ken", "Email": "ken@acme.org", "Manager": "Hiro"},
    {"Name": "Jo", "Email": "jo@acme.org", "Manager": "Hiro"},
    {"Name": "Ky", "Email": "ky@acme.org", "Manager": "Jane"},
]
```

## License

Copyright (C) 2024-2026 the Perf360 Authors

This is free software: you can redistribute it and/or modify them
under the terms of the GNU General Public License as published by the
Free Software Foundation, either version 3 of the License, or (at your
option) any later version.

This is distributed in the hope that it will be useful, but WITHOUT
ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License
for more details.

You should have received a copy of the GNU General Public License
in gpl.txt. If not, see <http://www.gnu.org/licenses>.
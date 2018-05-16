# mq-metric-samples

This repository contains a collection of IBM® MQ sample clients that utilize the [IBM® MQ golang metric packages](https://github.com/ibm-messaging/mq-golang) to provide a program that can be used with existing monitoring technologies (such as prometheus, AWS Cloud watch, etc). These samples were moved to this repository from the [IBM® MQ golang metric packages repository](https://github.com/ibm-messaging/mq-golang).

## Health Warning

This package is provided as-is with no guarantees of support or updates. There are also no guarantees of compatibility with any future versions of the package; the API is subject to change based on any feedback.

These samples also use the **in-development** version of the `mqmetric` and `ibmmq` golang pacakges. The indevelopment version is found in the [dev branch of the mq-golang repository](https://github.com/ibm-messaging/mq-golang/tree/dev). If you wish to use the stable versions then please use samples found the [v1.0.0 release of the mq-golang GitHub repository](https://github.com/ibm-messaging/mq-golang/tree/v1.0.0).

## Getting started

### Sample requirements

To build these samples you need to have prebuilt the mqmetric and ibmmq golang packages. You can find instructions on how to build these [here](https://github.com/ibm-messaging/mq-golang/tree/dev#getting-started). **Note:** You must use the [dev branch of the mq-golang repostiory](https://github.com/ibm-messaging/mq-golang/tree/dev) with this repository.

You will also require the following programs:

* [dep](https://golang.github.io/dep/) - Golang dependency management tool

### Building a component

* Change directory to your go path. (`cd $GOPATH`)
* Use git to get a copy of this repository into a new directory in the workspace:

  ```git clone https://github.com/ibm-messaging/mq-metric-samples.git src/github.com/ibm-messaging/mq-metric-samples```

* Navigate to the mq-metric-samples root directory (`$GOPATH/src/github.com/ibm-messaging/mq-metric-samples`)
* Run dep to ensure that you have the correct dependencies downloaded:

```dep ensure```

* Compile the sample program you wish to use. For example to build the mq_prometheus sample:

```go install cmd/mq_prometheus```

At this point, you should have a compiled copy of the code in `$GOPATH/bin`.

## History

See [CHANGELOG](CHANGELOG.md) in this directory.

## Issues and Contributions

For feedback and issues relating specifically to this package, please use the [GitHub issue tracker](https://github.com/ibm-messaging/mq-golang/issues).

Contributions to this package can be accepted under the terms of the IBM Contributor License Agreement,
found in the [CLA file](CLA.md) of this repository. When submitting a pull request, you must include a statement stating
you accept the terms in the CLA.

## Copyright

© Copyright IBM Corporation 2016, 2018
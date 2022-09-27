@echo off

rem Don't affect parent environment
setlocal

rem Set a PATH to include a suitable gcc build
set PATH=c:\tdm-gcc-64\bin;%PATH%

rem Go to the root of my Go tree
rem Usually something like c:\Gowork
cd %GOPATH%

rem And set references to the commands that have been git cloned under it
set R=%GOPATH%\src\github.com\ibm-messaging\mq-metric-samples
set D=%R%\cmd

cd %R%
echo Working from %R%

rem Make sure the correct size is going to be used - The invoked gcc may not set it
set CGO_CFLAGS=-D_WIN64

rem Do the builds - since we can't always use *.go, then name the files explicitly
rem Some have slightly different sets of files
for %%M in (mq_prometheus ) do (
echo Building %%M
go build -mod=vendor -o bin/%%M.exe %D%\%%M\config.go %D%\%%M\main.go %D%\%%M\exporter.go %D%\%%M\status.go
type %R%\config.common.yaml %D%\%%M\config.collector.yaml > bin\%%M.yaml 2>NUL:
)

for %%M in (mq_json mq_coll mq_influx) do (
echo Building %%M
go build -mod=vendor -o bin/%%M.exe %D%\%%M\config.go %D%\%%M\main.go %D%\%%M\exporter.go
type %R%\config.common.yaml %D%\%%M\config.collector.yaml > bin\%%M.yaml 2>NUL:
)

for %%M in (mq_aws mq_opentsdb) do (
echo Building %%M
go build -mod=vendor -o bin/%%M.exe %D%\%%M\config.go %D%\%%M\main.go %D%\%%M\exporter.go %D%\%%M\points.go
type %R%\config.common.yaml %D%\%%M\config.collector.yaml > bin\%%M.yaml 2>NUL:
)

endlocal

@echo off
set DATA_SOURCE_NAME=postgresql://postgres:adminp@localhost:5432/tickleveldb?sslmode=disable
C:\tools\postgres_exporter\postgres_exporter.exe

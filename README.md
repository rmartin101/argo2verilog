# argo2verilog
An Argo to Verilog Compiler

# Introduction

FPGAs were born to emulate hardware circuits. But modern FPGAs are more than just circuit emulators - they are a unique computing environment. A modern FPGA is a sea of registers, look-up-tables, memories, and hard arthimetic overlayed by a programmible interconnect and synchronous clocks. This computing environment has not been fully exploited because the the current software stacks are built to support hardware emulation. 

Argo is a non-strict subset of the Go language language designed with abstractions that are simple to understand and also map to the strengths of FPGAs. An Argo programmer's mental model compiles into and FPGA straightforwardly, much as a C program compiles targets a CPU in a straightforward manner. 

# Why build on Go?

Go is a popular language that supports Communicating Sequential Process (CSP) contructs directly in the language. CSP languages map well into FPGAs for two reasons. First, the channel abstraction maps to an FPGA's interconnect and FIFOs. Second, the CSP parallelization model of many small, asynchronously communicating processes map well into the FPGAs 2D sea of registers, LUTs and arithmetic units. 

Another reason to use Go is that Argo programs can use the existing Go compiler and run-time. This allows Argo programs written in a CSP style to break up the parallelism between the CPU and an FPGA in a seamless manner as some processes are allocated to the CPU and other to the FPGA. 

# How is Argo different from Go?

Argo stands for Array Go. In Argo, arrays are a primitive type. Array operators will be natively supported, similar to languages like Matlab or ZPL. Making arrays first class objects allows the compiler to reduce them the systolic style of computation that FPGA are good at. When compiling Argo to Go, arrays are constructed as a syntactic sugar. That is, array operations are reduced to library calls. 

Argo does not support dymanic memory management. All modern programming languages support the abstraction of a single large store from which we can draw memory, recycle it and return it. (e.g. new/malloc, garbage collection, and free). The large memory pool abstraction is a poor fit for FPGAs. All variables and channels in Argo must be declared at compile-time, which enables the compiler to map them into an FPGA's registers and BRAMs. Dynamic allocation and recycling do not fit well into a sea of small memories. 

# Getting started:

## Dependencies: 
  Go, Antlr4, iverilog 

## Install the Antr4 parser 

## Install the iverlog simulator 

## Install the Go compiler. 

## Clone the Repository 

## Make and run the examples 


















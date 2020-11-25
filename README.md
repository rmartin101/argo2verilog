# argo2verilog
An Argo to Verilog Compiler

# What is Argo?

FPGAs were born to emulate hardware circuits. But modern FPGAs are more than just circuit emulators - they are a unique computing environment. A modern FPGA is a sea of registers, look-up-tables, memories, and hard arthimetic overlayed by a programmible interconnect and synchronous clocks. This computing environment has not been fully exploited because the the current software stacks are built to support hardware emulation. 

**Argo is a non-strict subset of the Go language language designed to run on FPGAs and CPUs.** The language abstractions are simple to understand and map to the strengths of FPGAs. An Argo programmer's mental model maps into an FPGA straightforwardly, much as C program translates onto a CPU in a natural manner. Argo is designed to easliy re-use Go's compiler and run-time for debugging and performance testing. 

## Why build on Go?

Go is a popular language that supports Communicating Sequential Process (CSP) contructs directly in the language. CSP languages map well into FPGAs for two reasons. First, the channel abstraction maps to an FPGA's interconnect and FIFOs. Second, the CSP parallelization model of many small, asynchronously communicating processes map well into the FPGAs 2D sea of registers, LUTs and arithmetic units. 

Another reason to build on Go written is a CSP-style is that Argo programs can use the existing Go compiler and run-time. This allows Argo programs to allocate computation between the CPU and an FPGA in a seamless manner within the same program. Some processes are allocated to the CPU and others to the FPGA. 

## How is Argo different from Go?

**First Class Arrays.** Argo stands for Array Go. In Argo, arrays are a primitive type. Array operators will be natively supported, similar to languages like Matlab or APL. Making arrays first class objects allows the compiler to reduce them the systolic style of computation that FPGA are good at. When compiling Argo to Go, arrays are constructed as a syntactic sugar. That is, array operations are reduced to library calls, thus allowing Argo programs to re-use the Go compiler and runtime. 

**Static Memory Management.** Argo does not support dymanic memory management. All modern programming languages support the abstraction of a single large store from which we can draw memory, recycle it, and return it, I.e., new/malloc, garbage collection, and free. The large memory pool abstraction is a poor fit for FPGAs. All variables and channels in Argo must be declared at compile-time, which enables the compiler to map them into an FPGA's registers and BRAMs. Dynamic allocation and recycling do not fit well into a grid of small memories. 

# Getting started:

First, get all the dependencies. Then git clone the repository. Finally, build and run the examples. 

## Dependencies: 

Argo2verilog depends on 3 packages: Golang, Antlr4, iverilog. 

Golang is the Go compiler and run-time. [Antlr4](https://www.antlr.org/) is system used to build the Argo lexer and parser. [Icarus Verilog](http://iverilog.icarus.com/) is the Verilog run time used. Future work will port to [Verilator](https://www.veripool.org/wiki/verilator).

## Install the Antr4 lexer and parser 

## Install the iverlog simulator 

## Install the Go compiler

## Clone the Argo2verilog Repository 

## Make and run the examples 

### channel_simple.go 

### forloops.go 


















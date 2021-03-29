
ANTLR4=~/bin/antlr4 

# # Good intro to makefiles: 
# # 
# https://www3.ntu.edu.sg/home/ehchua/programming/cpp/gcc_make.html

all: argo2verilog.go genVerilog.go
	${ANTLR4} -Dlanguage=Go -o parser Argo.g4
	go build -gcflags 'all=-N -l' argo2verilog.go genVerilog.go 

check: ../test/forstatements.go ../test/forstatements.go ../test/ifstatements.go ../test/channel01.go 
	../bin/argo2verilog -check -i ../test/forstatements.go
	../bin/argo2verilog -check -i ../test/ifstatements.go
	../bin/argo2verilog -check -i ../test/channel01.go 

simple: ../test/simple_if.go
	./argo2verilog -i ../test/simple_if.go -o ./simple_if.v
	iverilog -o ./simple_if.vvp ./simple_if.v

install: argo2verilog 
	cp argo2verilog ../bin

.PHONY: clean run

clean:
	rm argo2verilog	

run:
	./argo2verilog -gv -i ../test/channel01.go






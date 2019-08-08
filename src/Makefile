
ANTLR4=~/bin/antlr4 

all: argo2verilog.go 
	${ANTLR4} -Dlanguage=Go -o parser Argo.g4
	go build argo2verilog.go


install: argo2verilog 
	cp argo2verilog ../bin

.PHONY: clean

clean:
	rm argo2verilog	

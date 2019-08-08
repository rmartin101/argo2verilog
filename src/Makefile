
ANTLR4=~/bin/antlr4 

all:
	${ANTLR4} -Dlanguage=Go -o parser Argo.g4
	go build argo2verilog.go


.PHONY: clean

clean:
	rm argo2verilog	

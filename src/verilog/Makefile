# Argo to Verilog Compiler: Makefile to test Verilog templates 
#    (c) 2020, Richard P. Martin and contributers 
#    
#    This program is free software: you can redistribute it and/or modify
#    it under the terms of the GNU General Public License as published by
#    the Free Software Foundation, either version 3 of the License, or
#    (at your option) any later version.
#
#    This program is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
#    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#    GNU General Public License Version 3 for more details.
#
#    You should have received a copy of the GNU General Public License
#    along with this program.  If not, see <https://www.gnu.org/licenses/>

all: argo_3stage_bench.vvp argo_packet_bench.vvp

argo_3stage_bench.vvp: argo_3stage_bench.v argo_3stage.v argo_queue.v d_p_ram.v 
	iverilog -g2005-sv -o argo_3stage_bench.vvp argo_3stage_bench.v argo_3stage.v argo_queue.v d_p_ram.v

argo_packet_bench.vvp: argo_packet_bench.v argo_3stage.v argo_queue.v d_p_ram.v
	iverilog -g2005-sv -o argo_packet_bench.vvp argo_packet_bench.v argo_packet.v argo_queue.v d_p_ram.v

.PHONY: clean run

clean:
	rm *.vvp 

run:
	vvp argo_3stage_bench.vvp
732-

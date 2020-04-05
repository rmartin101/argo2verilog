
/* Argo to Verilog Compiler: Verilog Templates 
    (c) 2020, Richard P. Martin and contributers 
    
    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License Version 3 for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>
*/

/* a simple dual ported RAM */
/* this should be inferrable by allmost all the tools into a BRAM */
module d_p_ram #(parameter ADDR_WIDTH = 3, DATA_WIDTH = 32, DEPTH = (1<< ADDR_WIDTH))
                (clock, write_en, write_addr, read_addr, input_data, output_data);

/* port definitions */    
   input wire 			clock;
   input wire 			write_en;
   input wire [ADDR_WIDTH-1:0] 	write_addr;
   input wire [ADDR_WIDTH-1:0] 	read_addr;
   input wire [DATA_WIDTH-1:0] 	input_data;
   output reg [DATA_WIDTH-1:0] 	output_data ;

   reg [DATA_WIDTH-1:0] memory [0:DEPTH-1]; 

   always @ (posedge clock) begin
      if(write_en) begin
         memory[write_addr] <= input_data;
      end
      output_data <= memory[read_addr];
   end
endmodule

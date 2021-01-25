/*  Example program to compute an FFT using a CSP style of design 
 *
    (c) 2021, Richard P. Martin and contributers 
    
    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License Version 3 for more details.t

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>
*/


/* This code implements an FFT using a CSP style of design in Go
*  The program build a 2D butterfly. The nodes in the graph are Goroutines and the
*  edges are channels. Nodes are labeled in column-major order 
*    
input I(0)    ->  (0,0)--->(1,0)--->(2,0)--->(3,0)  -->output  X(0)
           \ /  
              ->  (0,1)--->(1,1)--->(2,1)--->(3,1)  -->output  X(1)
           \ /    
           ->  (0,2)--->(1,2)--->(2,2)--->(3,2)  -->output  X(2)
           / \ 
           ->  (0,3)--->(1,3)--->(2,3)--->(3,3)  -->output  X(3)       
*/
package main;

import ("fmt"); 

func node(col uint32,row uint32, in1 chan float64, in2 chan float64, out1 chan float64, out2 chan float64, omega float64, control chan uint8 ) {
	var a,b,value float64;
	var msg uint8;
	var quit bool;

	quit = false;

	// while quit == false 	
	for (quit == false) {
		
		a = <- in1;
		b = <- in2;
	
		value = a + omega*b;

		out1 <- value;
		out2 <- value;

		// poll the control channel 
		if (len(control) > 0) {
			msg = <- control;
			fmt.Printf("node %d:%d got control message %d \n",col,row,msg)
			if (msg == 0xFF) {
				fmt.Printf("node %d:%d ending \n",row,col,msg)
				return ; 
			}; 
		}; 
	}; 
} ;

func main() {

	// make an array of channels 
	// levels*(2^levels)  8*3 = 24 channels 
	//call a go routing wih node I having channel 

	const FFT_LOG uint32 = 3 ;  // log of the number of inputs/outputs
	const FFT_LOG1 uint32 = (FFT_LOG+1)  // the nubmber of stages/columns of the FFT
	const FFT_VSIZE uint32 = (1<<FFT_LOG) ;  // size of the input vector 
	const FFT_NODES uint32 = (FFT_VSIZE * (FFT_LOG + 1) )  ; // number of nodes
	// each node has 2 inputs + 2 outputs, but outputs are shared as inputs
	// except at the edges, which are the length of a vector 
	const FFT_CHANNELS uint32 = (FFT_NODES *2) + (FFT_VSIZE*2)
	var r,c int32; // have to be ints because used in loops counting backwards, need to go negative

	var straight_channels [FFT_LOG][FFT_VSIZE]   chan float64;
	var cross_channels [FFT_LOG][FFT_VSIZE] chan float64;
	var output_channels [FFT_VSIZE]         chan float64;
	var cntl_channels  [FFT_LOG1][FFT_VSIZE] chan uint8;

	var cross_bit_location_in, cross_bit_location_out uint32 ;
	var cross_bit_in, cross_bit_out uint32 ;
	var cross_row_input, cross_row_output uint32; 
	
	fmt.Printf("fft sizes are %d %d %d \n", FFT_LOG, FFT_VSIZE, FFT_NODES) ; 

	// make all the channels. The outer loop indexes the rows. The inner loop indexes the columns
	// we can set the inputs and outputs by row in the first outer loop 
	for r = 0; r < int32(FFT_VSIZE) ; r++ {
		output_channels[r] =  make(chan float64);
		for c = 0; c < int32(FFT_LOG) ; c++ {
			straight_channels[c][r]= make(chan float64);
			cross_channels[c][r]= make(chan float64);
			cntl_channels[c][r] = make(chan uint8, 1);			
		}
	}

	// this is the code that creates the butterfly
	// recall an N-input FFT butterfly has log N levels
	// every level is a vector size. e.g. an 8 element FFT has 3 levels and each
	// vector length is 8; a 16 input FFT has 4 levels and the number of nodes
	// in each level is 16 

	// each node is a go-routine connected by channels
	// loop for every level of the FFT, from outputs to inputs.
	// and create the nodes with the right channel interconnect

	// this version of the loop counts backwards from the outputs
	for r = int32(FFT_VSIZE-1) ; r >=0 ; r-- {
		for c = int32(FFT_LOG-1); c >=0 ; c-- {

			// get the target row for the cross input channel 
			cross_bit_location_in  = (1<<c)  // bit-location for the input channel
			cross_bit_location_out = (1<<c+1)  // the output channel is one column to the left 
			
			cross_bit_in = uint32(r)|cross_bit_location_in  // the actual bit location
			cross_bit_out = uint32(r)|cross_bit_location_out 

			// target row for the cross input channel 
			if (cross_bit_location_in == 0) {
				cross_row_input = cross_bit_in + uint32(r)
			} else {
				cross_row_input = cross_bit_in - uint32(r)
			}

			// target row for the cross output channel 
			if (cross_bit_location_out == 0) {
				cross_row_output = cross_bit_out + uint32(r)
			} else {
				cross_row_output = cross_bit_out - uint32(r)
			}

			// the output column is special, no outputs to other nodes 
			if (c == int32(FFT_LOG-1)) {
				/*go node(c,r,straight_channel[c][r],cross_channel[(1<<(c-1))+r],
					output_channels[r],nil,1,cntl_channel[c][r]);
                                 */
				fmt.Printf("Creating output node(%d:%d) inputs: (%d:%d,%d:%d) output: %d \n",c,r,c,r,c,cross_row_input,r)
			} else {	
				fmt.Printf("Creating node(%d:%d) inputs: (%d:%d,%d:%d) outputs (%d:%d,%d:%d) \n",c,r,c,r,c,cross_row_input,c+1,r,c+1,cross_row_output)
			}
		}
		fmt.Printf(" \n ");			
	}
	
}
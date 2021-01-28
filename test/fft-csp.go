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
import ("math");

func node(col uint32,row uint32, in1 chan complex128, in2 chan complex128, out1 chan complex128, out2 chan complex128, Wn complex128, control chan uint8 ) {
	var a,b,value complex128;
	var msg uint8;
	var quit bool;

	quit = false;

	// while quit == false 	
	for (quit == false) {
		
		a = <- in1;
		b = <- in2;
	
	        value = a + Wn*b;
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

// for an FFT of size N, get the twiddle factor for a node at column c, row r 
func compute_twiddle_factor(col,row uint32) complex128 {
	var m,N uint32; 
	var inner float64;
	var retval complex128;

	N = (1<<(col+1))-1;   // factors on the unit circle for a N-node FFT (2^col)-1
	m = row % (N+1);  // need a bit-mask here, but use mod for now 
	
	// recall e^-i*2*Pi*m/N = cos(2*Pi*m/N) - i*sin(2*Pi*m/N)
	inner = 2.0*math.Pi*float64(m)/float64(N) ;
	retval = complex(math.Cos(inner),-1.0*math.Sin(inner)) ;
	return retval;
}

func create_fft_array() {
	// make an array of channels 

	const FFT_LOG uint32 = 3 ;  // log of the number of inputs/outputs
	const FFT_LOG1 uint32 = (FFT_LOG+1)  // the nubmber of stages/columns of the FFT
	const FFT_VSIZE uint32 = (1<<FFT_LOG) ;  // size of the input vector 
	const FFT_NODES uint32 = (FFT_VSIZE * (FFT_LOG + 1) )  ; // number of nodes
	// each node has 2 inputs + 2 outputs, but outputs are shared as inputs
	// except at the edges, which are the length of a vector 
	const FFT_CHANNELS uint32 = (FFT_NODES *2) + (FFT_VSIZE*2)
	var r,c uint32; // the current column of row of the node to create 

	var straight_channels [FFT_LOG][FFT_VSIZE]   chan complex128;
	var cross_channels [FFT_LOG][FFT_VSIZE] chan complex128;
	var output_channels [FFT_VSIZE]         chan complex128;
	var cntl_channels  [FFT_LOG1][FFT_VSIZE] chan uint8;
	var twiddle complex128
	
	// these are use to compute the target row for the cross channels in the butterfly
	var cross_distance_in, cross_distance_out uint32 ; // number of rows from current row
	var cross_bit_value_in, cross_bit_value_out uint32;  // direction 0=decreasing (up) 1=down
	var cross_row_input, cross_row_output uint32;  // the actual target row 

	
	fmt.Printf("fft sizes are %d %d %d \n", FFT_LOG, FFT_VSIZE, FFT_NODES) ; 

	// make all the channels. The outer loop indexes the rows. The inner loop indexes the columns
	// we can set the inputs and outputs by row in the first outer loop 
	for r = 0; r < FFT_VSIZE ; r++ {
		output_channels[r] =  make(chan complex128);
		for c = 0; c < FFT_LOG ; c++ {

			straight_channels[c][r]= make(chan complex128);
			cross_channels[c][r]= make(chan complex128);
			cntl_channels[c][r] = make(chan uint8, 1);
		}
	}

	// this is the code that creates the butterfly using go routines and  channels 
	// recall an N-input FFT butterfly has log N levels
	// every level is a vector with a size. e.g. an 8 element FFT has 3 levels and each
	// vector length is 8; a 16 input FFT has 4 levels and the number of nodes
	// in each level/vector is 16
	// We use this terminology to organize a butterfly as a 2D array
	// each level (or stage) is a column and the vectors element number is the row
	// So a node is defined as a column-major array with nodes labeled as (c,r)
	// the 0th column is the input and the LogNth column is the output
	

	// each node is a go-routine connected by channels
	// loop for every level of the FFT, from outputs to inputs.
	// and create the nodes with the right channel interconnect

	// main loop to create compute nodes. For each column, for each row, create the node with
	// the correct set of channels. 
	for c = 0; c < FFT_LOG ; c++ {   // we have a log(fftsize) columns 
		for r = 0; r < FFT_VSIZE ; r++ {  // vector size rows 

			// get the distance from the current row to the target row for the cross input channel 
			cross_distance_in  = (1<<c)  // distance from current row for the input
			cross_distance_out = (1<<(c+1))  // the output channel is one column to the right, so the distance is larger

			// the value of the bit in the row number determines if the offset is up or down
			cross_bit_value_in = r & cross_distance_in
			cross_bit_value_out = r & cross_distance_out
			
			// target row for the cross input channel 
			if (cross_bit_value_in == 0) {
				cross_row_input = r +  cross_distance_in
			} else {
				cross_row_input = r - cross_distance_in
			}

			// target row for the cross output channel 
			if (cross_bit_value_out == 0) {
				cross_row_output = r + cross_distance_out
			} else {
				cross_row_output = r - cross_distance_out
			}

			twiddle = compute_twiddle_factor(c,r)
			// we have to special case the input and output vectors 
			if (c == 0) { // input column 
				fmt.Printf("Creating input node(%d:%d) inputs: (%d,%d) outputs:  (%d:%d,%d:%d) twid:%.3f\n",c,r, r,cross_row_input, c+1,r, c+1,cross_row_output,twiddle)
			} else if (c == (FFT_LOG-1)) {  //output column 
				/*go node(c,r,straight_channel[c][r],cross_channel[(1<<(c-1))+r],
					output_channels[r],nil,1,cntl_channel[c][r]);
                                 */
				fmt.Printf("Creating output node(%d:%d) inputs: (%d:%d,%d:%d) output: %d twid:%.3f \n",c,r,c,r,c,cross_row_input,r,twiddle)
			} else { // interior columns/nodes
				fmt.Printf("Creating node(%d:%d) inputs: (%d:%d,%d:%d) outputs (%d:%d,%d:%d) twid:%.3f\n",c,r,c,r,c,cross_row_input,c+1,r,c+1,cross_row_output,twiddle)
			}
		}
		fmt.Printf(" \n ");			
	}
	
}

func read_outputs() {

}

func main() {

	create_fft_array()

}

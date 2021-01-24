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

package main;

import ("fmt"); 

func node(id int, in1 chan float64, in2 chan float64, out1 chan float64, out2 chan float64, weight float64, control chan uint8 ) {
	var a,b,value float64; 
	var quit bool;

	quit = false;
	
	for (quit == false) { // while quit == false 
		
		a = <- in1;
		b = <- in2;
	
		value = a + weight*b;

		out1 <- value;
		out2 <- value;
		
		if (len(control) > 0) {
			msg = <- control;
			fmt.Printf("node %d got control message %d \n",id,control)
		}; 
	}; 
} ;

func main() {

	// make an array of channels 
	// levels*(2^levels)  8*3 = 24 channels 
	//call a go routing wih node I having channel 

	const FFT_LOG uint32 = 3 ;  // log of the number of inputs/outputs 
	const FFT_VSIZE uint32 = (1<<FFT_LOG) ;  // size of the input vector 
	const FFT_NODES uint32 = (FFT_VSIZE * FFT_LOG) ; // number of nodes
	// each node has 2 inputs + 2 outputs, but outputs are shared as inputs
	// except at the edges, which are the length of a vector 
	const FFT_CHANNELS uint32 = (FFT_NODES *2) + (FFT_VSIZE*2)
	var i int;
	
	var all_channels [FFT_CHANNELS]chan float64;
	var all_controls [FFT_NODES] chan bool;
	
	fmt.Printf("fft sizes are %d %d %d \n", FFT_LOG, FFT_VSIZE, FFT_NODES) ; 

	// make all the data channels, no buffering 
	for (i = 0; i< FFT_CHANNELS; i++) {
		all_channels[i]= make(chan float64);
	}

	// make all the control channels, no buffering 
	for (i = 0; i< FFT_NODES; i++) {
		all_controls[i]= make(chan uint8);
	}

	// make the done channels, which terminate the nodes
	// 

	// this is the code that creates the butterfly
	// recall an N-input FFT butterfly has log N levels
	// every level is a vector size. e.g. an 8 element FFT has 3 levels and each
	// vector length is 8; a 16 input FFT has 4 levels and the number of nodes
	// in each level is 16 

	// each node is a go-routine connected by channels
	// loop for every level of the FFT, from outputs to inputs.
	// and create the nodes with the right channel interconnect

	// this is the output vector 
	for ( i=0; i< FFT_VSIZE; i++) {
		go node(i,all_channels);
	}

	// these are the inner layers/vectors 
	for ( i=0; i< FFT_VSIZE-2; i++) {
	}

	// this is the input vector/layer 
	for ( i=0; i< FFT_VSIZE; i++) {
	}
	
}

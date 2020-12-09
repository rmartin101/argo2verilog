// Small program to test IF statements 

package main ;

import ( "fmt" ) ;
import ( "net" ) ;

func blammo(i,j int) int {
	if(i <= j) {
		return i*j ;
	} ;
	return i+j ;
} ;

func blargo(i int) (int,int) {
	return i+i, i*i ;
} ;


type UDPpacket struct {
	id int; 
	protocol int;       
};


func main() {
	var i,j,k int ; 
	
	i = 1 ;
	j = 2 ; 
	k = 3 ;

	addr := net.UDPAddr{net.ParseIP("127.0.0.1"), 3003,"", };

	addr2 := net.UDPAddr{net.ParseIP("127.0.0.1"), 3003, "", };

	
	// simple if statement 
	if  (i < j) {
		fmt.Printf("I is less than J \n") ; 
	} ;

	
	// if with an else 
	if  (k >= (i + 3)) {
		i = 4;
		k = i + i;
		//  blargo_i_param_0 = j 
		//  control to blargo
		//  i  = blargo_retval_0
		//  j =  blargo_retval_1
		k,i = blargo(j); 
		fmt.Printf("K is: %d \n", k) ;
	} else {
		i = 4;
		j = i + 3 ;
		if (j == 7) {
			fmt.Printf("I and J are:: %d %d  \n", k,j) ;
		};
	} ;

	// if with a simple statement at the begining
	if  x:=3; k <= (i + blammo(i,j) + blammo(j,i) )  {
		fmt.Printf(" X is %d  \n", x) ;
		
	};
	
	// if else with simple statement 
	if  y:= 0xFFAB ; y <= ( i+j+k )  {
		fmt.Printf("Y is %d  \n", y) ;
		
	} else {
		fmt.Printf("Y*Y is %d  \n", y*y) ;	
	} ;
	// highly chained if-elses 
	if  z:=7; z > ( z*(i*i) * (j+k) )  {
		fmt.Printf(" Z is %d  \n", z) ; 
	} else if (( z * z) > 3) {
		fmt.Printf("Z*Z is %d  \n", z*z) ;
	} else if ( (z - i ) > 4) {
		fmt.Printf("Z*Z*Z is %d  \n", z*z*z) ;	
	} else {
		fmt.Printf("End of the chained if \n") ;
	} ;

	k = i+j;
} ;

/* 

 if () { 
    <body1> 
 } else <body2> if () { 
    <body3>
 } else <body4> if () { 
   <body5>
 } else <body6> if () 
  <body6>
 }
 eos 

*/

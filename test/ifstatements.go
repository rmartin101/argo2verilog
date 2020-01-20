// Small program to test IF statements 

package main ;

import ( "fmt" ) ;


func main() {
	var i,j,k int ; 

	i = 1 ;
	j = 2 ; 
	k = 3 ;

	// simple if statement 
	if  (i < j) {
		fmt.Printf("I is less than J \n") ;		
	} ;


	// if with an else 
	if  (k >= (i + 3)) {
		i = 4;
		k = i + i;
		fmt.Printf("K is: %d \n", k) ;
	} else {
		j = 7 ;
		fmt.Printf("I and J are:: %d %d  \n", k,j) ;
	} ;


	// if with a simple statement at the begining
	if  x:=3; k <= (i + x)  {
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
	}; 

}


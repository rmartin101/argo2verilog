// very simple IF statement test 

package main ;

import ( "fmt" ) ;


func main() {
	var i,j,k,z int ; 

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
		fmt.Printf("I and J are:: %d %d  \n", k,j) ;
	} ;

	// highly chained if-elses
	z = i+j+k ;
	if   ( (j+k) < z )  {
		z = 0;
		fmt.Printf(" Z is %d  \n", z) ; 
	} else if (( z * z) > 3) {
		fmt.Printf("Z*Z is %d  \n", z*z) ;
	} else {
		fmt.Printf("End of the chained if \n") ;
	} ;

	k = i+j;
} ;


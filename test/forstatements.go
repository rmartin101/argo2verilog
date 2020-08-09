// small program to test for statements


package main ;

import ( "fmt" ) ;

func snafu(i,j int) int {
	var a int ;
	a = i;
	if (i <= j) {
		return i*j*a ;
	} ;

	return i*j - (i-j) ;
} ;

func plusOne(i int) int {
	for i < 3 {
		i = i +1;
		i = i * 2;
	};

	return i + 1; 
};

func main() {
	var i,j,k int ; 
	
	i = 1 ;
	j = 2 ; 
	k = 4 ;

	sum := 0x0000; 
	for i = 1; i < 5 ; i = i + 1 {
		for j = 1; j < 3; j++ {
			for z:= 0; z < k; z++ {
				sum = sum + j  ;
				j = snafu(j,k);
			}; 
		};
	}; 
	fmt.Printf("The sum is %d \n",sum) ;

	j =  1;

	for j < 5 {
		j = j*2;
	};
	
	fmt.Printf("j is %d\n",j) ; 

	sum =0 ;
	for {
		sum = sum +1;
		if (sum > 4) {
			break;
		}; 
		fmt.Printf("Inside the sum is %d \n",sum) ;	
	};
	fmt.Printf("Outer sum is %d \n",sum) ;
	sum = plusOne(-1);
	sum = 0;
	for i := plusOne(i); i < 7 ; i = i + 1 {
		sum = sum + i;
		if (sum >3) {
			continue;
		};
		fmt.Printf("Looped sum is %d \n",sum) ;		
	};

	for i:= 0; i< 10; i++ {
		fmt.Printf("I is local to this for statement %d \n",i) ;		
	};
	
	fmt.Printf("I is local to this for the main function %d \n",i) ;
	
	fmt.Printf("Final sum is %d \n",sum) ;
	fmt.Printf("End of Program \n") ;	
} ;


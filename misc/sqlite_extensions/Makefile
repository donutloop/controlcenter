all: helpers.so

helpers.so: test_helpers.c
	gcc -o $@ $^ -g -fPIC -shared

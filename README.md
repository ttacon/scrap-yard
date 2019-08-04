# scrap-yard
Collection of tools and code thrown together to answer questions


## node-wastage

Quick tool thrown together on a train ride to figure out why so much disk
space was being used in a directory that shouldn't have been using so much
space. Turns out 8-9GB were all duplicated node modules (at the top level,
this script doesn't even look at nested levels yet).

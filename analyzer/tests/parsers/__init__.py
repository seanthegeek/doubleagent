"""Tests of the parsers and of the helpers they share.

Most classes build the synthetic home of `fixtures.build_home` through
`ParserBase`, open it as an input and check the rows one parser yields: their
columns, the thinking opt-in, the history files, the files the parser must not
want and the rows of a bad line. The fixtures follow the record shapes in the
Parsing part of each `research/<agent>.md`.
"""

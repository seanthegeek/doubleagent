// Package urlcredentials finds the credentials of URLs inside longer text
// (a command line, JSON, Markdown), where net/url parses one URL, by these
// rules; those citing RFC 3986 follow it, and in doubt they find more:
//
//  1. Text splits into tokens at white space, double quotes and angle
//     brackets (appendix C), and at backticks; nothing is read across them.
//  2. A URL starts at a scheme, a letter then letters, digits, +, - and .
//     (section 3.1), directly followed by "://" or JSON's escaped `:\/\/`.
//  3. A single quote, a ), ] or } that closes no bracket opened earlier in
//     its parameter (rule 6), a comma directly before a scheme, and . , : ;
//     ! ? before one of them or the token's end end the value being read
//     (rule 7) but not the reading: rules 4 to 6 go on.
//  4. The authority follows the separator and ends at the first /, ? or #
//     (section 3.2), a backslash, the next URL's scheme or the token's end.
//  5. The user information is what stands before the authority's last @
//     (section 3.2.1; the last is this package's choice). With a colon it
//     (user:password) is a credential whole, and its password one too,
//     unless it is the colon alone; without, a name (Credential.NameOnly).
//  6. The query runs from the first ? or # after the first URL's authority
//     (sections 3.4 and 3.5) to the token's end. In it a name starts after
//     each &, ;, ?, #, JSON's \u0026 or \U0026, and backslash not before /.
//     Each but ? and # ends a value and a parameter, as does the first #,
//     where the fragment (#access_token=A&token_type=bearer) begins and what
//     a client sends ends. A query may hold ? and a fragment holds no #, so a
//     later # joins a hash route to a fragment (#/cb#access_token=A) or is a
//     token's own; like ?, it ends neither, so no tail of a token shows, and
//     rule 3 counts brackets across it. A name inside a value is part of it.
//  7. A parameter's value, after its first =, is a credential when it is not
//     empty and its name, percent-escapes decoded and ASCII case ignored, is
//     one credentialParameters lists.
package urlcredentials

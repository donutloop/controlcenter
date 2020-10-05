/*
 * This file aims to provide SQLite extensions that we can dynamically load
 * and help us to test the queries we use in the Go code, but from other SQLite
 * based applications (including the sqlite3 REPL)
*/

#include "sqlite3ext.h"
SQLITE_EXTENSION_INIT1
#include <assert.h>
#include <string.h>

static void resolve_domain_mapping(
  sqlite3_context *context,
  int argc,
  sqlite3_value **argv
){
  assert( argc==1 );
  // NOTE: this code is not doing any domain mapping at all,
  // but just providing something like the Go implementation in /domainmapping
  // just so that we can use the same queries in the sqlite prompt
  sqlite3_result_value(context, argv[0]);
}

#ifdef _WIN32
__declspec(dllexport)
#endif
int sqlite3_helpers_init(
  sqlite3 *db, 
  char **pzErrMsg, 
  const sqlite3_api_routines *pApi
){
  int rc = SQLITE_OK;
  SQLITE_EXTENSION_INIT2(pApi);
  (void)pzErrMsg;
  rc = sqlite3_create_function(db, "lm_resolve_domain_mapping", 1,
                     SQLITE_UTF8 | SQLITE_DETERMINISTIC,
                     0, resolve_domain_mapping, 0, 0);
  return rc;
}

"""Failure-first installed Playwright and actual executable identity."""
import copy
import unittest
import campaign_r06_restore_runtime_tools as tools

class RestoreToolControls(unittest.TestCase):
    def registry(self):
        return {"browsers":[{"name":"chromium","revision":"1234","browserVersion":"1.2"},
                            {"name":"chromium-headless-shell","revision":"1234","browserVersion":"1.2"}]}

    def test_actual_chromium_and_headless_registry_paths_both_required(self):
        result=tools.browser_paths(self.registry(), "/owned/browser-cache")
        self.assertEqual(result["chromium"],"/owned/browser-cache/chromium-1234/chrome-linux64/chrome")
        self.assertEqual(result["headless"],"/owned/browser-cache/chromium_headless_shell-1234/chrome-headless-shell-linux64/chrome-headless-shell")

    def test_missing_duplicate_or_foreign_registry_revision_rejects(self):
        values=[{"browsers":[]}, {"browsers":self.registry()["browsers"][:1]},
                {"browsers":self.registry()["browsers"]+[self.registry()["browsers"][0]]}]
        for revision in ("../outside","1234/escape",1234,"",True):
            value=self.registry();value["browsers"][0]["revision"]=revision;values.append(value)
        for value in values:
            with self.assertRaises(ValueError): tools.browser_paths(value,"/owned/browser-cache")

    def test_relative_cache_override_blocks(self):
        for cache in ("relative","/owned/../outside","/owned\0cache"):
            with self.assertRaises(ValueError): tools.browser_paths(self.registry(),cache)

    def state(self):
        value={"bytes":100,"sha256":"a"*64,"gitBlob":"b"*40,"mode":0o755}
        return {"go":dict(value),"node":dict(value),"pnpm":dict(value),"chromium":dict(value),"headless":dict(value)}

    def test_five_actual_executable_records_not_versions_are_required(self):
        value=self.state();self.assertTrue(tools.valid_tools(value))
        for key in value:
            bad=self.state();bad.pop(key);self.assertFalse(tools.valid_tools(bad))
        bad=self.state();bad["go"]={"version":"go1.27"}
        self.assertFalse(tools.valid_tools(bad))

    def test_nonexecutable_noninteger_and_oversized_records_reject(self):
        for change in ({"mode":0o600},{"bytes":True},{"bytes":0},{"bytes":1024*1024*1024},
                       {"sha256":"A"*64},{"extra":"private"}):
            value=self.state();value["chromium"].update(change)
            self.assertFalse(tools.valid_tools(value))

    def test_before_after_executable_or_dependency_closure_drift_blocks(self):
        before={"executables":self.state(),"installedClosure":{"bytes":100,"sha256":"c"*64,"files":5}}
        self.assertTrue(tools.same_state(before,copy.deepcopy(before)))
        for key in ("go","node","pnpm","chromium","headless"):
            after=copy.deepcopy(before);after["executables"][key]["sha256"]="d"*64
            self.assertFalse(tools.same_state(before,after))
        after=copy.deepcopy(before);after["installedClosure"]["sha256"]="d"*64
        self.assertFalse(tools.same_state(before,after))

    def test_symlink_nonregular_or_unstable_tool_capture_rejects(self):
        for mode in ("symlink","nonregular","unstable"):
            def reader(*args,**kwargs): raise ValueError(mode)
            with self.assertRaises(ValueError): tools.capture_tool("/owned/tool",reader=reader)

    def package_candidate(self, path, name):
        return {"manifest": path+"/package.json", "root": path,
                "package": {"name": name, "version": "1.63.0"}}

    def resolved(self, layout):
        base="/owned/repo/apps/subtitles/e2e"
        if layout=="flat":
            test=base+"/node_modules/@playwright/test"; playwright=base+"/node_modules/playwright"; core=base+"/node_modules/playwright-core"
        elif layout=="pnpm":
            store="/owned/repo/node_modules/.pnpm"
            test=store+"/@playwright+test@1.63.0/node_modules/@playwright/test"
            playwright=store+"/playwright@1.63.0/node_modules/playwright"
            core=store+"/playwright-core@1.63.0/node_modules/playwright-core"
        else:
            test=base+"/node_modules/@playwright/test"
            playwright=test+"/node_modules/playwright"; core=playwright+"/node_modules/playwright-core"
        return {name:self.package_candidate(path,name) for name,path in
                (("@playwright/test",test),("playwright",playwright),("playwright-core",core))}

    def test_flat_isolated_pnpm_and_nested_transitive_package_roots(self):
        for layout in ("flat","pnpm","nested"):
            value=self.resolved(layout)
            roots=tools.package_roots(value,"/owned/repo")
            self.assertEqual(set(roots),{"@playwright/test","playwright","playwright-core"})
            self.assertEqual(str(roots["playwright-core"]),value["playwright-core"]["root"])

    def test_malformed_or_escaped_nearest_package_candidate_blocks(self):
        for label in ("@playwright/test","playwright","playwright-core"):
            for change in ({"root":"/outside"}, {"manifest":"/outside/package.json"},
                           {"root":"relative"}, {"root":"/owned/repo/../outside"},
                           {"package":{"name":"foreign","version":"1.63.0"}},
                           {"package":{"name":label,"version":"1.62.0"}}, {"extra":"private"}):
                value=self.resolved("pnpm");value[label].update(change)
                with self.assertRaises(ValueError): tools.package_roots(value,"/owned/repo")
        for label in ("@playwright/test","playwright","playwright-core"):
            value=self.resolved("flat");value.pop(label)
            with self.assertRaises(ValueError): tools.package_roots(value,"/owned/repo")

    def test_cli_and_registry_use_selected_importer_roots(self):
        self.assertIn("@playwright/test/package.json",tools.RESOLVE)
        self.assertIn("createRequire(testManifest)",tools.RESOLVE)
        self.assertIn("createRequire(playwrightManifest)",tools.RESOLVE)
        roots=tools.package_roots(self.resolved("nested"),"/owned/repo")
        command=tools.selected_cli(roots,"/owned/node",["test","--list"])
        self.assertEqual(command,["/owned/node",str(roots["@playwright/test"])+"/cli.js","test","--list"])
        registry=tools.registry_script(roots["playwright-core"])
        self.assertIn(str(roots["playwright-core"])+"/lib/coreBundle.js",registry)
        self.assertNotIn("/lib/server/registry/index.js",registry)
        self.assertNotIn("createRequire",registry)

    def test_controls_dependencies_are_go_only_with_no_optional_stale_rows(self):
        go={"go":self.state()["go"]}
        self.assertTrue(tools.valid_tools(go,"restore-controls"))
        self.assertFalse(tools.valid_tools(self.state(),"restore-controls"))
        self.assertFalse(tools.valid_tools({},"restore-controls"))
        self.assertFalse(tools.valid_tools({"node":self.state()["node"]},"restore-controls"))
        before={"executables":go,"installedClosure":None}
        self.assertTrue(tools.same_state(before,copy.deepcopy(before),"restore-controls"))
        for closure in ({}, {"files":[]}, {"sha256":"a"*64}):
            bad={**before,"installedClosure":closure}
            self.assertFalse(tools.same_state(bad,copy.deepcopy(bad),"restore-controls"))

    def test_browser_dependency_modes_reject_missing_or_foreign_keys(self):
        for suite in ("restore-headers","restore-inspect-body"):
            self.assertTrue(tools.valid_tools(self.state(),suite))
            self.assertFalse(tools.valid_tools({"go":self.state()["go"]},suite))
            value=self.state();value["python"]=dict(value["go"])
            self.assertFalse(tools.valid_tools(value,suite))
        self.assertFalse(tools.valid_tools(self.state(),"save-headers"))

    def bounded_scan(self, count, directories=False, swap=False, close_error=False):
        import stat
        from types import SimpleNamespace
        from unittest.mock import patch
        import campaign_r06_restore_runtime_sources as sources
        retained=[0];scanned=[];closed=[];opens=[]
        class Entry:
            def __init__(self,index):self.name="entry-"+str(index);self.path="/unowned/path/"+self.name
            def stat(self,follow_symlinks=False):
                return SimpleNamespace(st_mode=stat.S_IFDIR if directories else stat.S_IFREG,st_dev=1,st_ino=2)
        class Scan:
            def __init__(self,target):self.target=target;self.index=0
            def __iter__(self):return self
            def __next__(self):
                if self.index>=count if self.target in ("/owned/root",7) else self.index>=0:raise StopIteration
                self.index+=1;retained[0]+=1
                if retained[0]>5001:raise RuntimeError("fictional scan never bounded")
                return Entry(self.index)
            def close(self):
                closed.append("iterator")
                if close_error:raise OSError("fictional iterator close")
            def __enter__(self):return self
            def __exit__(self,*args):self.close()
        info=SimpleNamespace(st_mode=stat.S_IFDIR,st_dev=1,st_ino=2,st_mtime_ns=3,st_ctime_ns=4)
        def scan(target):scanned.append(target);return Scan(target)
        def opened(path,flags,**kwargs):
            opens.append((path,flags,kwargs))
            if kwargs and swap:raise OSError("fictional child symlink swap")
            return 7 if not kwargs else 8
        with patch.object(tools.os,"scandir",side_effect=scan),patch.object(tools.os,"lstat",return_value=info), \
             patch.object(tools.os,"open",side_effect=opened),patch.object(tools.os,"fstat",return_value=info), \
             patch.object(tools.os,"close",side_effect=lambda fd:closed.append(fd)), \
             patch.object(tools,"read_bytes",return_value=b"x"),patch.object(tools.time,"monotonic",return_value=0):
            try:tools.installed_closure({"playwright":"/owned/root"})
            except (ValueError,OSError,RuntimeError):pass
            else:self.fail("unsafe or partial closure accepted")
        return retained[0],scanned,closed,opens

    def test_excessive_entries_are_bounded_before_retention_and_sort(self):
        retained,scanned,closed,opens=self.bounded_scan(5002)
        self.assertLessEqual(retained,5001)
        self.assertTrue(all(type(target) is int for target in scanned))
        self.assertIn("iterator",closed);self.assertIn(7,closed)

    def test_pending_directories_are_bounded_before_materialization(self):
        retained,scanned,closed,opens=self.bounded_scan(5000,directories=True)
        self.assertLessEqual(retained,129)
        self.assertIn(7,closed)

    def test_directory_symlink_swap_and_iterator_close_failure_block(self):
        retained,scanned,closed,opens=self.bounded_scan(1,directories=True,swap=True)
        self.assertTrue(any(kwargs.get("dir_fd")==7 and flags & tools.os.O_NOFOLLOW for _,flags,kwargs in opens))
        self.assertIn(7,closed)
        retained,scanned,closed,opens=self.bounded_scan(1,close_error=True)
        self.assertIn("iterator",closed);self.assertIn(7,closed)

    def test_regular_package_files_use_owned_directory_descriptor(self):
        import stat
        from types import SimpleNamespace
        from unittest.mock import patch
        class Entry:
            name="owned.js";path="/unowned/escape.js"
            def stat(self,follow_symlinks=False):return SimpleNamespace(st_mode=stat.S_IFREG,st_dev=1,st_ino=2)
        class Scan:
            def __iter__(self):return iter([Entry()])
            def close(self):pass
            def __enter__(self):return self
            def __exit__(self,*args):self.close()
        info=SimpleNamespace(st_mode=stat.S_IFDIR,st_dev=1,st_ino=2,st_mtime_ns=3,st_ctime_ns=4)
        def read(path,limit,**kwargs):
            self.assertEqual(path,"owned.js");self.assertEqual(kwargs.get("dir_fd"),7)
            return b"x"
        with patch.object(tools.os,"open",return_value=7),patch.object(tools.os,"scandir",return_value=Scan()), \
             patch.object(tools.os,"fstat",return_value=info),patch.object(tools.os,"lstat",return_value=info), \
             patch.object(tools.os,"close") as closed,patch.object(tools,"read_bytes",side_effect=read),patch.object(tools.time,"monotonic",return_value=0):
            value=tools.installed_closure({"playwright":"/owned/root"})
        self.assertEqual(value["files"][0]["path"],"owned.js")
        closed.assert_called_once_with(7)

class RestoreDependencyDiagnosticControls(unittest.TestCase):
    def test_only_closed_dependency_reason_values_are_exported(self):
        for reason in ("source-shape", "source-parent", "tool-unavailable", "tool-resolution"):
            self.assertEqual(tools.dependency_reason(ValueError(reason)), reason)
        for error in (ValueError("private path /unreviewed/value"), ValueError("source-shape", "private"),
                      OSError("private target"), RuntimeError("private text")):
            self.assertEqual(tools.dependency_reason(error), "unclassified")

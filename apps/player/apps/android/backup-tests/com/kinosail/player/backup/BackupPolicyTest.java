package com.kinosail.player.backup;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;
import static org.robolectric.util.ReflectionHelpers.ClassParameter.from;

import android.app.backup.BackupAgent;
import android.app.backup.BackupDataInput;
import android.app.backup.BackupDataOutput;
import android.app.backup.FullBackupDataOutput;
import android.content.Context;
import android.content.pm.ApplicationInfo;
import android.content.res.XmlResourceParser;
import android.os.ParcelFileDescriptor;
import android.os.Process;
import android.os.UserHandle;
import java.io.File;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.robolectric.Robolectric;
import org.robolectric.RobolectricTestRunner;
import org.robolectric.RuntimeEnvironment;
import org.robolectric.android.controller.BackupAgentController;
import org.robolectric.annotation.Config;
import org.robolectric.annotation.Implementation;
import org.robolectric.annotation.Implements;
import org.robolectric.util.ReflectionHelpers;
import org.xmlpull.v1.XmlPullParser;

/** Framework selection regression; no OEM transport or device-transfer claim. */
@RunWith(RobolectricTestRunner.class)
@Config(sdk = 35, shadows = BackupPolicyTest.TarRecorder.class)
public class BackupPolicyTest {
    private static final int CLOUD = 0;
    private static final int DEVICE_TRANSFER = 1;

    @Test public void cloudRulesExcludeEveryCanary() throws Exception {
        assertEquals(Collections.emptySet(), selectedCanaries(CLOUD, false));
    }

    @Test public void deviceTransferRulesExcludeEveryCanary() throws Exception {
        assertEquals(Collections.emptySet(), selectedCanaries(DEVICE_TRANSFER, false));
    }

    @Test public void noRulesControlSelectsAllNineDomains() throws Exception {
        Set<String> domains = Set.of("root", "file", "database", "sharedpref",
            "device_root", "device_file", "device_database", "device_sharedpref", "external");
        assertEquals(domains, selectedCanaries(CLOUD, true));
        assertEquals(domains, selectedCanaries(DEVICE_TRANSFER, true));
    }

    private Set<String> selectedCanaries(int destination, boolean noRules) throws Exception {
        Context app = RuntimeEnvironment.getApplication();
        ApplicationInfo info = app.getApplicationInfo();
        int originalRules = ReflectionHelpers.getField(info, "dataExtractionRulesRes");
        int rules = compiledBackupRules(app);
        int legacyRules = ReflectionHelpers.getField(info, "fullBackupContent");
        assertEquals(0, info.flags & ApplicationInfo.FLAG_ALLOW_BACKUP);
        Map<String, String> canaries = seedCanaries(app);
        Class<?> fullBackup = Class.forName("android.app.backup.FullBackup");
        Map<?, ?> schemeCache = ReflectionHelpers.getStaticField(fullBackup, "kPackageBackupSchemeMap");
        BackupAgentController<DefaultAgent> controller = Robolectric.buildBackupAgent(DefaultAgent.class);
        schemeCache.clear();
        TarRecorder.paths.clear();
        try {
            // Robolectric's legacy PackageParser omits dataExtractionRules. Bind the
            // compiled manifest's actual reference, without replacing backup parsing.
            ReflectionHelpers.setField(info, "dataExtractionRulesRes", noRules ? 0 : rules);
            if (noRules) {
                ReflectionHelpers.setField(info, "fullBackupContent", 0);
            }
            DefaultAgent agent = controller.create().get();
            ReflectionHelpers.callInstanceMethod(agent, "onCreate",
                from(UserHandle.class, Process.myUserHandle()), from(int.class, destination));
            assertEquals(destination, (int) ReflectionHelpers.getField(agent, "mBackupDestination"));
            int flags = destination == DEVICE_TRANSFER ? BackupAgent.FLAG_DEVICE_TO_DEVICE_TRANSFER : 0;
            FullBackupDataOutput output = ReflectionHelpers.callConstructor(FullBackupDataOutput.class,
                from(long.class, -1L), from(int.class, flags));
            agent.onFullBackup(output);
            Set<String> selected = new TreeSet<>();
            canaries.forEach((domain, path) -> {
                if (TarRecorder.paths.contains(path)) selected.add(domain);
            });
            System.out.println("backup-policy destination=" + destination + " control=" + noRules
                + " selectedCanaries=" + selected);
            return selected;
        } finally {
            controller.destroy();
            ReflectionHelpers.setField(info, "dataExtractionRulesRes", originalRules);
            ReflectionHelpers.setField(info, "fullBackupContent", legacyRules);
            schemeCache.clear();
            TarRecorder.paths.clear();
        }
    }

    private int compiledBackupRules(Context app) throws Exception {
        String android = "http://schemas.android.com/apk/res/android";
        int applications = 0;
        int rules = 0;
        try (XmlResourceParser manifest = app.getAssets().openXmlResourceParser("AndroidManifest.xml")) {
            while (manifest.next() != XmlPullParser.END_DOCUMENT) {
                if (manifest.getEventType() != XmlPullParser.START_TAG) continue;
                if (manifest.getName().equals("manifest")) {
                    assertEquals(app.getPackageName(), manifest.getAttributeValue(null, "package"));
                }
                if (manifest.getName().equals("application")) {
                    applications++;
                    assertFalse(manifest.getAttributeBooleanValue(android, "allowBackup", true));
                    rules = manifest.getAttributeResourceValue(android, "dataExtractionRules", 0);
                    assertTrue("The compiled manifest must reference backup rules", rules != 0);
                    assertEquals("xml", app.getResources().getResourceTypeName(rules));
                    assertEquals("data_extraction_rules", app.getResources().getResourceEntryName(rules));
                }
            }
        }
        assertEquals("The compiled manifest must have one application", 1, applications);
        return rules;
    }

    private Map<String, String> seedCanaries(Context app) throws IOException {
        Context credential = ReflectionHelpers.callInstanceMethod(app, "createCredentialProtectedStorageContext");
        Context device = app.createDeviceProtectedStorageContext();
        assertFalse(credential.getDataDir().getCanonicalPath().equals(device.getDataDir().getCanonicalPath()));
        Map<String, String> files = new LinkedHashMap<>();
        seedStorage(credential, "", files);
        seedStorage(device, "device_", files);
        seed("external", new File(app.getExternalFilesDir(null), "backup-canary.txt"), files);
        assertEquals(9, new TreeSet<>(files.values()).size());
        return files;
    }

    private void seedStorage(Context context, String prefix, Map<String, String> files) throws IOException {
        seed(prefix + "root", new File(context.getDataDir(), "backup-canary.txt"), files);
        seed(prefix + "file", new File(context.getFilesDir(), "backup-canary.txt"), files);
        seed(prefix + "database", context.getDatabasePath("backup-canary.db"), files);
        File preferences = ReflectionHelpers.callInstanceMethod(context, "getSharedPreferencesPath",
            from(String.class, "backup-canary"));
        seed(prefix + "sharedpref", preferences, files);
    }

    private void seed(String domain, File file, Map<String, String> files) throws IOException {
        Files.createDirectories(file.toPath().getParent());
        Files.write(file.toPath(), "backup-policy-canary".getBytes(StandardCharsets.UTF_8));
        assertTrue(file.isFile());
        files.put(domain, file.getCanonicalPath());
    }

    public static class DefaultAgent extends BackupAgent {
        @Override public void onBackup(ParcelFileDescriptor oldState, BackupDataOutput data,
            ParcelFileDescriptor newState) { throw new AssertionError("Unexpected key/value backup"); }
        @Override public void onRestore(BackupDataInput data, int version,
            ParcelFileDescriptor newState) { throw new AssertionError("Unexpected restore"); }
    }

    @Implements(className = "android.app.backup.FullBackup", isInAndroidSdk = false)
    public static class TarRecorder {
        static final Set<String> paths = new TreeSet<>();

        @Implementation protected static int backupToTar(String packageName, String domain,
            String linkDomain, String rootPath, String path, FullBackupDataOutput output) {
            paths.add(path);
            return 0;
        }
    }
}

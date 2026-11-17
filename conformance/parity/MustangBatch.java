// MustangBatch validates files with Mustang's ZUGFeRDValidator in a single
// JVM, and writes the XML result of each to <out dir>/<file name>.xml.
// Mustang's command line starts a JVM and compiles the schematrons for every
// file; this runs them once. Used by conformance/parity.sh:
//
//	java -cp Mustang-CLI-2.26.0.jar MustangBatch.java <out dir> <list of files>
//
// Notices are disabled, as with the --no-notices option of the command line:
// Mustang would otherwise run the XRechnung schematron, as notices, on every
// EN 16931 invoice.

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;

import org.mustangproject.validator.ZUGFeRDValidator;

public class MustangBatch {
	public static void main(String[] args) throws Exception {
		Path out = Paths.get(args[0]);
		Files.createDirectories(out);
		for (String line : Files.readAllLines(Paths.get(args[1]), StandardCharsets.UTF_8)) {
			if (line.isBlank()) {
				continue;
			}
			Path in = Paths.get(line.trim());
			String report;
			try {
				ZUGFeRDValidator validator = new ZUGFeRDValidator();
				validator.disableNotices();
				report = validator.validate(in.toString());
			} catch (Throwable e) {
				report = "<validation><xml><exception type=\"0\">" + escape(e.toString()) + "</exception></xml></validation>";
			}
			Files.writeString(out.resolve(in.getFileName() + ".xml"), report, StandardCharsets.UTF_8);
		}
	}

	private static String escape(String s) {
		return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;");
	}
}

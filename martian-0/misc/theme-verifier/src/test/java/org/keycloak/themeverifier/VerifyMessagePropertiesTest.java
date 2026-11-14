/*
 * Copyright 2025 Red Hat, Inc. and/or its affiliates
 * and other contributors as indicated by the @author tags.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package org.keycloak.themeverifier;

import org.apache.maven.plugin.MojoExecutionException;
import org.hamcrest.MatcherAssert;
import org.hamcrest.Matchers;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.File;
import java.net.URL;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

class VerifyMessagePropertiesTest {

    @Test
    void verifyDuplicateKeysDetected() throws MojoExecutionException {
        List<String> verify = getFile("duplicateKeys_en.properties").verify();
        MatcherAssert.assertThat(verify, Matchers.hasItem(Matchers.containsString("Duplicate keys in file")));
    }

    @Test
    void verifyIllegalHtmlTagDetected() throws MojoExecutionException {
        List<String> verify = getFile("illegalHtmlTag_en.properties").verify();
        MatcherAssert.assertThat(verify, Matchers.hasItem(Matchers.containsString("Illegal HTML")));
    }

    @Test
    void verifyNoHtmlAllowed() throws MojoExecutionException {
        List<String> verify = getFile("noHtml_de.properties").verify();
        MatcherAssert.assertThat(verify, Matchers.hasItem(Matchers.containsString("Illegal HTML")));
    }

    @Test
    void verifyNoChangedAnchors() throws MojoExecutionException {
        List<String> verify = getFile("changedAnchor_de.properties").verify();
        MatcherAssert.assertThat(verify, Matchers.hasItem(Matchers.containsString("Didn't find anchor tag")));
    }

    @Test
    void verifyOverlappingPrefixAndSuffix(@TempDir Path directory) throws IOException, MojoExecutionException {
        Path file = directory.resolve("overlap_en.properties");
        Files.writeString(file, "key=foo<script>foo\n");

        MatcherAssert.assertThat(new VerifyMessageProperties(file.toFile()).verify(),
                Matchers.hasItem(Matchers.containsString("'<script>foo' vs. ''")));
    }

    @Test
    void verifyOnlyAnchorsBypassHtmlPolicy(@TempDir Path directory) throws IOException, MojoExecutionException {
        Path file = directory.resolve("anchors_en.properties");
        Files.writeString(file, "anchor=<a href=\"https://example.com\">link</a>\n"
                + "abbreviation=<abbr>text</abbr>\n"
                + "article=<article>text</article>\n"
                + "custom=<a-custom>text</a-custom>\n");

        List<String> verify = new VerifyMessageProperties(file.toFile()).verify();
        MatcherAssert.assertThat(verify, Matchers.hasSize(3));
        for (String key : List.of("abbreviation", "article", "custom")) {
            MatcherAssert.assertThat(verify, Matchers.hasItem(Matchers.containsString("Illegal HTML in key " + key)));
        }
    }

    private static VerifyMessageProperties getFile(String fixture) {
        URL resource = VerifyMessageProperties.class.getResource("/" + fixture);
        if (resource == null) {
            throw new RuntimeException("Resource not found: " + fixture);
        }
        return new VerifyMessageProperties(new File(resource.getFile()));
    }

}

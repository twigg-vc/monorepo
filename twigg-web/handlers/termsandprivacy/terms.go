package termsandprivacy

import "net/http"

func handleGetTerms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	const tos = `Terms of Service
Effective Date: October 28, 2025
Operator: Twigg (https://twigg.vc/)

1. Acceptance of Terms
By creating an account, accessing, or using Twigg (the “Service”), you agree to be bound by these Terms of Service (“Terms”). If you do not agree to these Terms, you must not access or use the Service.
Twigg reserves the right to update or modify these Terms at any time without prior notice. Your continued use of the Service after any modification constitutes acceptance of the revised Terms.

2. Description of Service
Twigg provides a code hosting and version control service accessible via a web interface and command-line tool (the “CLI”). Users may store, manage, and share code repositories and related data through the Service.
Twigg may modify, suspend, or discontinue any part of the Service at any time, with or without notice, and shall not be liable for any resulting loss or inconvenience.

3. Eligibility and Accounts
You must be at least 18 years old and legally capable of entering into binding agreements to use the Service.
You are responsible for maintaining the confidentiality of your account credentials and for all activity occurring under your account.

Twigg reserves the right to suspend or terminate accounts at its sole discretion, for any reason, including suspected misuse or violation of these Terms.

4. User Content and Data
You retain ownership of any code, data, or materials (“Content”) you upload or store on Twigg. By uploading Content, you grant Twigg a worldwide, non-exclusive, royalty-free license to store, display, and process the Content solely as necessary to provide and maintain the Service.
Twigg does not guarantee the availability, integrity, or security of any Content. Users are solely responsible for maintaining backups of their data.
Twigg assumes no liability for loss, corruption, or unauthorized access to any Content.

5. Acceptable Use
You agree not to use the Service for any unlawful purpose or in any way that may damage, disable, or impair the operation of the Service.
 You must not upload or distribute any Content that:
-Violates any applicable law or regulation;
-Infringes any intellectual property rights;
-Contains malicious code, viruses, or harmful software.

Twigg reserves the right, but not the obligation, to review, remove, or restrict access to any Content at its sole discretion.

6. Payment and Subscription
Access to the Service requires payment of applicable fees. All payments are non-refundable unless explicitly stated otherwise by Twigg.
Twigg may change its pricing and payment terms at any time. Continued use of the Service after any such change constitutes acceptance of the new terms.
Failure to pay may result in suspension or termination of your account and loss of access to your data.

7. Service Availability and Disclaimer of Warranties
The Service is provided on an “AS IS” and “AS AVAILABLE” basis, without any warranties of any kind, express or implied, including, without limitation, warranties of merchantability, fitness for a particular purpose, or non-infringement.
Twigg does not warrant that:
-The Service will be uninterrupted, secure, or error-free;
-Any data will be preserved, stored, or backed up reliably;
-Any defects will be corrected.

You acknowledge that use of the Service is at your own risk.

8. Limitation of Liability
To the maximum extent permitted by law, Twigg, its owners, employees, and affiliates shall not be liable for any direct, indirect, incidental, special, consequential, or exemplary damages, including but not limited to loss of profits, data, goodwill, or other intangible losses, resulting from:
-Your use or inability to use the Service;
-Any unauthorized access to or alteration of your data;
-Any third-party conduct or content on the Service;
-Any modification, suspension, or discontinuation of the Service.

In no event shall Twigg’s total liability exceed the amount you paid for access to the Service during the twelve (12) months preceding the claim.

9. Indemnification
You agree to indemnify, defend, and hold harmless Twigg, its owners, employees, and affiliates from and against any and all claims, damages, liabilities, losses, and expenses (including reasonable attorneys’ fees) arising out of your use of the Service, your Content, or your violation of these Terms.

10. Termination
Twigg may terminate or suspend your account or access to the Service at any time, without notice, for any reason, including violation of these Terms.
Upon termination, your right to use the Service immediately ceases. Twigg shall have no obligation to retain or provide any data after termination.

11. Governing Law
These Terms shall be governed by and construed in accordance with the laws of Brazil, without regard to conflict of law principles.
Any disputes arising from or related to these Terms or the Service shall be resolved exclusively in the courts located in São Paulo, Brazil.

12. Entire Agreement
These Terms constitute the entire agreement between you and Twigg concerning the use of the Service and supersede all prior or contemporaneous communications, proposals, and agreements, whether oral or written.

13. Contact
For any questions regarding these Terms, please contact:
marcos@twigg.vc
`

	_, _ = w.Write([]byte(tos))
}

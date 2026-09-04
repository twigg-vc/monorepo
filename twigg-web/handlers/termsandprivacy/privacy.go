package termsandprivacy

import "net/http"

func handleGetPrivacy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	const privacy = `Privacy Policy
Effective Date: October 28, 2025
Operator: Twigg (https://twigg.vc/)

1. Introduction
Twigg (“we”, “us”, “our”) provides a version control and code hosting platform (the “Service”). This Privacy Policy explains how we collect, use, store, and disclose information when you use the Service.

By accessing or using Twigg, you agree to the collection and use of information in accordance with this Policy. If you do not agree, you must stop using the Service immediately.

2. Information We Collect
We may collect the following categories of information:
a. Account Information
When you create an account, we collect information such as your name, email address, username, password, and billing information.
b. Usage and Technical Data
We may automatically collect technical data, including IP addresses, device identifiers, operating system details, browser information, access times, and activity logs.
c. User Content
Any code, repositories, text, images, files, or other materials you upload, store, or share through Twigg (“Content”) are collected and processed solely for the purpose of operating the Service.
d. Payment Information
If you make payments, billing data is processed by our third-party payment processor. Twigg does not store or handle your full credit card or banking information directly.
e. Communications
We may collect copies of communications you send to us, including support requests, bug reports, and feedback.

3. How We Use Information
We may use the collected information to:
Operate, maintain, and improve the Service;
-Authenticate users and manage accounts;
-Provide customer support and respond to inquiries;
-Process payments and manage subscriptions;
-Send administrative notifications and important service updates;
-Detect and prevent fraud, abuse, or illegal activity;
-Comply with legal obligations.

Twigg does not sell or rent user data to third parties.

4. Data Retention
We retain user data for as long as necessary to provide the Service, fulfill legal obligations, or resolve disputes.
We may delete inactive accounts or stored data at our discretion and without notice.
Users are solely responsible for maintaining their own backups of any data or repositories stored on Twigg.

5. Data Security
We use commercially reasonable measures to protect your data. However, no system is completely secure, and we cannot guarantee the absolute security of your data.
You acknowledge that:
-The Service may experience outages, intrusions, or data loss;
-Transmission of data over the Internet is inherently insecure;
-You use the Service and store data at your own risk.
Twigg shall not be liable for any unauthorized access, loss, or alteration of data.

6. Data Sharing
We may share information in the following limited circumstances:
-Service Providers: With trusted vendors that perform services on our behalf (e.g., hosting, billing). These parties are required to process information only as directed by us.
-Legal Compliance: When required by law, subpoena, or other legal process, or to protect Twigg’s rights or property.
-Business Transfers: In connection with any merger, acquisition, or sale of assets, provided the receiving entity agrees to honor the commitments of this Policy.
-Public Content: If you choose to make repositories or content public, that information becomes accessible to others.

Twigg disclaims any responsibility for the use of publicly available information.

7. International Transfers
Twigg may process and store data on servers located in various countries. By using the Service, you consent to the transfer of your information outside your country, including to jurisdictions that may not provide the same level of data protection.

8. Your Responsibilities
You are responsible for:
Ensuring that any information you provide is accurate and lawful;
-Maintaining appropriate backups of your data;
-Obtaining necessary permissions for any third-party content you upload;
-Complying with all applicable privacy laws and regulations in your jurisdiction.

9. Your Rights
Depending on your jurisdiction, you may have certain rights related to your personal data, such as the right to access, correct, delete, or restrict its processing.
Requests regarding personal data can be submitted to marcos@twigg.vc.
Twigg reserves the right to verify identity and deny requests that are excessive, repetitive, or technically infeasible.

10. Children’s Privacy
The Service is not intended for use by individuals under 18 years of age. We do not knowingly collect personal information from minors. If we become aware that we have collected information from a minor, we will take reasonable steps to delete it.

11. Changes to This Policy
Twigg may modify this Privacy Policy at any time, at its sole discretion. Updated versions will be posted at https://twigg.vc/. Continued use of the Service after any modification constitutes acceptance of the revised Policy.

12. Disclaimer and Limitation of Liability
The Service is provided “AS IS” and “AS AVAILABLE” without warranties of any kind, express or implied.
 Twigg disclaims all liability for any damages arising from:
-Use or inability to use the Service;
-Loss or exposure of data;
-Security incidents, unauthorized access, or third-party actions;
-Any inaccuracies or omissions in this Policy.

You acknowledge and agree that Twigg's total liability under this Policy, if any, shall not exceed the amount paid by you for the Service during the twelve (12) months preceding the claim.

13. Governing Law
This Policy shall be governed by and construed in accordance with the laws of Brazil, without regard to conflict-of-law principles.
 Any disputes shall be resolved exclusively in the courts located in São Paulo, Brazil.

14. Contact
For privacy inquiries, contact:
marcos@twigg.vc
`
	_, _ = w.Write([]byte(privacy))
}
